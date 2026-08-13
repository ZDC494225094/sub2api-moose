import { applyManagedCanvasConfig, managedCanvasGroupID, type AiConfig, type ManagedCanvasGroup } from "@/stores/use-config-store";
import { reportInsufficientBalance } from "@/lib/canvas-billing";
import { useUserStore } from "@/stores/use-user-store";

const canvasAPIBase = "/api/v1/canvas";
const authTokenKey = "auth_token";
const refreshTokenKey = "refresh_token";
const tokenExpiresAtKey = "token_expires_at";
const authUserKey = "auth_user";

type RefreshTokenPair = {
    access_token: string;
    refresh_token: string;
    expires_in: number;
};

let refreshInFlight: Promise<boolean> | null = null;

export type CanvasRunImage = {
    url?: string;
    assetIndex?: number;
    mimeType?: string;
};

export type CanvasRunVideo = {
    url?: string;
    assetIndex?: number;
    mimeType?: string;
};

export type CanvasRunAudio = {
    assetIndex?: number;
    mimeType?: string;
};

export type CanvasRun = {
    id: string;
    status: "queued" | "running" | "succeeded" | "failed" | "canceled";
    content?: string;
    images?: CanvasRunImage[];
    videos?: CanvasRunVideo[];
    audios?: CanvasRunAudio[];
    error?: string;
};

export type CanvasRunPayload = Record<string, unknown> & { model: string };

export async function bootstrapManagedCanvas(signal?: AbortSignal) {
    const groups = await canvasRequest<ManagedCanvasGroup[]>("/config", { signal });
    applyManagedCanvasConfig(groups || []);
    return groups || [];
}

// modelSelection retains the channel-qualified model value (for example
// canvas:42::gpt-image-2) only while choosing the managed group. The payload
// itself must keep the plain provider model name expected by the gateway.
export async function startCanvasRun(config: AiConfig, payload: CanvasRunPayload, signal?: AbortSignal, modelSelection = payload.model) {
    const groupId = managedCanvasGroupID(config, modelSelection);
    if (!groupId) throw new Error("请选择可用分组中的模型");
    return canvasRequest<CanvasRun>("/runs", {
        method: "POST",
        body: JSON.stringify({ ...payload, groupId }),
        signal,
    });
}

export function getCanvasRun(id: string, signal?: AbortSignal) {
    return canvasRequest<CanvasRun>(`/runs/${encodeURIComponent(id)}`, { signal });
}

export async function waitForCanvasRun(config: AiConfig, payload: CanvasRunPayload, signal?: AbortSignal, modelSelection = payload.model, onRunStarted?: (runId: string) => void | Promise<void>) {
    const run = await startCanvasRun(config, payload, signal, modelSelection);
    await onRunStarted?.(run.id);
    return waitForExistingCanvasRun(run.id, signal, run);
}

export async function waitForExistingCanvasRun(id: string, signal?: AbortSignal, initialRun?: CanvasRun) {
    const run = initialRun || (await getCanvasRun(id, signal));
    for (;;) {
        if (run.status === "succeeded") {
            refreshCanvasBillingStatus();
            return run;
        }
        if (run.status === "failed" || run.status === "canceled") {
            const error = new Error(run.error || "画布任务未完成");
            reportInsufficientBalance(error);
            throw error;
        }
        await delay(1000, signal);
        const current = await getCanvasRun(run.id, signal);
        Object.assign(run, current);
    }
}

export async function canvasRunImageDataURLs(run: CanvasRun, signal?: AbortSignal) {
    const images = run.images || [];
    return Promise.all(
        images.map(async (image, index) => {
            if (image.assetIndex === undefined && image.url) return image.url;
            const assetIndex = image.assetIndex ?? index;
            const blob = await canvasAsset(run.id, "images", assetIndex, signal);
            return blobToDataURL(blob);
        }),
    );
}

export async function canvasRunVideoResult(run: CanvasRun, signal?: AbortSignal) {
    const video = run.videos?.[0];
    if (!video) throw new Error("画布任务没有返回视频");
    if (video.assetIndex === undefined && video.url) return { url: video.url, mimeType: video.mimeType };
    const blob = await canvasAsset(run.id, "videos", video.assetIndex ?? 0, signal);
    return { blob, mimeType: video.mimeType || blob.type || "video/mp4" };
}

export async function canvasRunAudioBlob(run: CanvasRun, signal?: AbortSignal) {
    const audio = run.audios?.[0];
    if (!audio) throw new Error("画布任务没有返回音频");
    const blob = await canvasAsset(run.id, "audio", audio.assetIndex ?? 0, signal);
    return blob.type.startsWith("audio/") ? blob : new Blob([blob], { type: audio.mimeType || "audio/mpeg" });
}

export async function cancelCanvasRun(id: string) {
    await canvasRequest<CanvasRun>(`/runs/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function canvasAsset(runID: string, kind: "images" | "videos" | "audio", index: number, signal?: AbortSignal) {
    const response = await canvasFetch(`/runs/${encodeURIComponent(runID)}/${kind}/${index}`, { signal });
    if (response.status === 401) throwAuthenticationError();
    if (!response.ok) throw new Error(await responseMessage(response));
    return response.blob();
}

async function canvasRequest<T>(path: string, init: RequestInit = {}) {
    const response = await canvasFetch(path, init);
    if (response.status === 401) throwAuthenticationError();
    if (!response.ok) {
        const error = new Error(await responseMessage(response));
        reportInsufficientBalance(error);
        throw error;
    }
    const payload = (await response.json()) as { code?: number | string; message?: string; data?: T } | T;
    if (payload && typeof payload === "object" && "code" in payload) {
        if (payload.code !== 0 && payload.code !== "0") {
            const error = new Error(payload.message || "请求失败");
            reportInsufficientBalance(error);
            throw error;
        }
        return payload.data as T;
    }
    return payload as T;
}

function refreshCanvasBillingStatus() {
    const { loadSubscriptions, loadUser } = useUserStore.getState();
    void loadUser();
    void loadSubscriptions();
}

async function canvasFetch(path: string, init: RequestInit = {}, retryAfterRefresh = true): Promise<Response> {
    const headers = new Headers(init.headers);
    const accessToken = localStorage.getItem(authTokenKey) || "";
    if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
    if (init.body) headers.set("Content-Type", "application/json");
    const response = await fetch(`${canvasAPIBase}${path}`, {
        ...init,
        headers,
    });
    if (response.status === 401 && retryAfterRefresh && (await refreshCanvasAuthToken(accessToken))) {
        return canvasFetch(path, init, false);
    }
    return response;
}

async function refreshCanvasAuthToken(failedAccessToken: string): Promise<boolean> {
    const latestToken = localStorage.getItem(authTokenKey) || "";
    if (latestToken && latestToken !== failedAccessToken) return true;

    if (!refreshInFlight) {
        refreshInFlight = requestCanvasTokenRefresh(failedAccessToken);
        void refreshInFlight.finally(() => {
            refreshInFlight = null;
        });
    }
    return refreshInFlight;
}

async function requestCanvasTokenRefresh(failedAccessToken: string): Promise<boolean> {
    const refreshToken = localStorage.getItem(refreshTokenKey) || "";
    if (!refreshToken) return false;

    try {
        const response = await fetch("/api/v1/auth/refresh", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ refresh_token: refreshToken }),
        });
        if (!response.ok) return hasNewerAccessToken(failedAccessToken);

        const payload = (await response.json()) as {
            code?: number | string;
            data?: RefreshTokenPair;
        };
        const tokens = payload.data;
        if ((payload.code !== 0 && payload.code !== "0") || !tokens?.access_token || !tokens.refresh_token) {
            return hasNewerAccessToken(failedAccessToken);
        }

        // A concurrent Sub2API tab may have already rotated this one-time refresh token.
        if (localStorage.getItem(refreshTokenKey) !== refreshToken) {
            return hasNewerAccessToken(failedAccessToken);
        }

        localStorage.setItem(authTokenKey, tokens.access_token);
        localStorage.setItem(tokenExpiresAtKey, String(Date.now() + Math.max(1, tokens.expires_in) * 1000));
        localStorage.setItem(refreshTokenKey, tokens.refresh_token);
        return true;
    } catch {
        return hasNewerAccessToken(failedAccessToken);
    }
}

function hasNewerAccessToken(failedAccessToken: string): boolean {
    const current = localStorage.getItem(authTokenKey) || "";
    return Boolean(current && current !== failedAccessToken);
}

function clearInvalidAuthSession() {
    localStorage.removeItem(authTokenKey);
    localStorage.removeItem(refreshTokenKey);
    localStorage.removeItem(tokenExpiresAtKey);
    localStorage.removeItem(authUserKey);
}

function throwAuthenticationError(): never {
    clearInvalidAuthSession();
    redirectToLogin();
    throw new Error("登录已失效，请重新登录");
}

function redirectToLogin() {
    if (window.location.pathname.startsWith("/login")) return;
    const redirect = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    window.location.assign(`/login?redirect=${encodeURIComponent(redirect)}`);
}

async function responseMessage(response: Response) {
    try {
        const payload = (await response.json()) as { message?: string; error?: { message?: string } | string };
        const nestedMessage = typeof payload.error === "string" ? payload.error : payload.error?.message;
        return payload.message || nestedMessage || `请求失败（${response.status}）`;
    } catch {
        return `请求失败（${response.status}）`;
    }
}

function delay(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        if (signal?.aborted) {
            reject(new DOMException("Aborted", "AbortError"));
            return;
        }
        const timer = window.setTimeout(resolve, ms);
        signal?.addEventListener(
            "abort",
            () => {
                window.clearTimeout(timer);
                reject(new DOMException("Aborted", "AbortError"));
            },
            { once: true },
        );
    });
}

function blobToDataURL(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error("无法读取生成的资源"));
        reader.readAsDataURL(blob);
    });
}
