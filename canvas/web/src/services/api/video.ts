import { isXiaowenVideoModel, normalizeXiaowenResolution } from "@/lib/xiaowen-video";
import axios from "axios";
import { nanoid } from "nanoid";

import i18n from "@/i18n";
import { dataUrlToBlob, dataUrlToFile, readFileAsDataUrl } from "@/lib/image-utils";
import { getMediaBlob, resolveMediaUrl, uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import { imageToDataUrl } from "@/services/image-storage";
import { boolConfig, buildSeedancePromptText } from "@/lib/seedance-video";
import { buildApiUrl, isManagedCanvasModel, modelOptionName, resolveModelRequestConfig, resolveModelScript, withLocalProxy, type AiConfig } from "@/stores/use-config-store";
import { runModelPlugin } from "./model-plugin";
import { canvasRunError, canvasRunVideoResult, getCanvasRun, startCanvasRun } from "./canvas";
import type { ReferenceImage } from "@/types/image";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";
import { assertVideoReferences, prepareVideoConfig, videoMode, videoProfile, videoRequestProtocol } from "@/lib/video-models";

type VideoResponse = { id: string; status?: string; error?: { message?: string }; url?: string; result_url?: string; video_url?: string; content?: { video_url?: string; url?: string } | null };
type ApiVideoResponse = VideoResponse | { code?: number | string; data?: VideoResponse | null; msg?: string; message?: string; error?: { message?: string } };
type SeedanceTask = {
    id: string;
    status?: "queued" | "running" | "succeeded" | "completed" | "failed" | "cancelled" | "expired";
    error?: { code?: string; message?: string } | null;
    content?: { video_url?: string; url?: string; last_frame_url?: string } | null;
    url?: string;
    result_url?: string;
    video_url?: string;
};
type ApiEnvelope<T> = T | { code?: number | string; data?: T | null; msg?: string; message?: string; error?: { message?: string } };
type RequestOptions = { signal?: AbortSignal; onRunStarted?: (runId: string) => void | Promise<void> };
type VideoMediaOptions = RequestOptions & { videos?: ReferenceVideo[]; audios?: ReferenceAudio[] };
const apiText = (key: string, options?: Record<string, unknown>) => i18n.t(`apiErrors.${key}`, options);

function normalizeVideoAspectRatio(value: string) {
    return value || "16:9";
}

export type VideoGenerationResult = { blob?: Blob; url?: string; mimeType?: string };
export type VideoGenerationTask = { id: string; provider: "openai" | "gemini" | "seedance" | "plugin" | "canvas" | "minimax" | "minimax-h3" | "kling" | "kling-v3" | "omni"; model: string; path?: string };
type GeminiInlineData = { bytesBase64Encoded: string; mimeType: string };
type GeminiVideoOperation = {
    name?: string;
    done?: boolean;
    error?: { message?: string };
    response?: { generateVideoResponse?: { generatedSamples?: Array<{ video?: { uri?: string } }> } };
};
export type VideoGenerationTaskState = { status: "pending" } | { status: "completed"; result: VideoGenerationResult } | { status: "failed"; error: string };

/** Results for scripted (plugin) video models, which run their own create+poll in one shot at task creation. */
const pluginVideoResults = new Map<string, VideoGenerationResult>();

function aiApiUrl(config: AiConfig, path: string) {
    return buildApiUrl(config.baseUrl, path);
}

function aiHeaders(config: AiConfig, contentType?: string) {
    return {
        Authorization: `Bearer ${config.apiKey}`,
        ...(contentType ? { "Content-Type": contentType } : {}),
    };
}

export async function requestVideoGeneration(config: AiConfig, prompt: string, references: ReferenceImage[] = [], videoReferences: ReferenceVideo[] = [], audioReferences: ReferenceAudio[] = [], options?: RequestOptions): Promise<VideoGenerationResult> {
    const task = await createVideoGenerationTask(config, prompt, references, videoReferences, audioReferences, options);
    return waitForVideoGenerationTask(config, task, options);
}

export async function waitForVideoGenerationTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationResult> {
    const delayMs = task.provider === "seedance" ? 5000 : 2500;
    for (;;) {
        if (options?.signal?.aborted) throw new DOMException("Aborted", "AbortError");
        const state = await pollVideoGenerationTask(config, task, options);
        if (state.status === "completed") return state.result;
        if (state.status === "failed") throw videoTaskFailed(state.error);
        await delay(delayMs, options?.signal);
    }
}

export function isVideoTaskFailed(error: unknown) {
    return error instanceof Error && error.name === "VideoTaskFailed";
}

function videoTaskFailed(message: string) {
    const error = new Error(message);
    error.name = "VideoTaskFailed";
    return error;
}

export async function createVideoGenerationTask(config: AiConfig, prompt: string, references: ReferenceImage[] = [], videoReferences: ReferenceVideo[] = [], audioReferences: ReferenceAudio[] = [], options?: RequestOptions): Promise<VideoGenerationTask> {
    config = prepareVideoConfig(config, references.length, videoReferences.length, audioReferences.length);
    assertVideoReferences(config, references, videoReferences, audioReferences);
    const selectedModel = (config.model || config.videoModel).trim();
    const requestConfig = resolveModelRequestConfig(config, selectedModel);
    if (isManagedCanvasModel(config, selectedModel)) return createManagedCanvasVideoTask(config, selectedModel, requestConfig.model, prompt, references, videoReferences, audioReferences, options);
    const script = resolveModelScript(config, selectedModel);
    if (script) return createPluginVideoTask(requestConfig, selectedModel, script, prompt, references, { ...options, videos: videoReferences, audios: audioReferences });
    assertVideoConfig(requestConfig, requestConfig.model);
    const protocol = videoRequestProtocol(config);
    if (protocol === "ark") {
        return createSeedanceTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, options);
    }
    if (protocol === "veo") return createGeminiVideoTask(requestConfig, selectedModel, prompt, references, { ...options, videos: videoReferences, audios: audioReferences });
    if (protocol !== "relay") return createNativeVideoTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, protocol, options);
    return createOpenAIVideoTask(requestConfig, selectedModel, prompt, references, { ...options, videos: videoReferences, audios: audioReferences });
}

export async function pollVideoGenerationTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    if (task.provider === "plugin") {
        const result = pluginVideoResults.get(task.id);
        return result ? { status: "completed", result } : { status: "failed", error: apiText("pluginVideoExpired") };
    }
    if (task.provider === "canvas") {
        const run = await getCanvasRun(task.id, options?.signal);
        if (run.status === "succeeded") return { status: "completed", result: await canvasRunVideoResult(run, options?.signal) };
        if (run.status === "failed" || run.status === "canceled") return { status: "failed", error: canvasRunError(run) };
        return { status: "pending" };
    }
    const requestConfig = resolveModelRequestConfig(config, task.model);
    assertVideoConfig(requestConfig, requestConfig.model);
    if (task.provider === "gemini") return pollGeminiVideoTask(requestConfig, task, options);
    if (["minimax", "minimax-h3", "kling", "kling-v3", "omni"].includes(task.provider)) return pollNativeVideoTask(requestConfig, task, options);
    return task.provider === "seedance" ? pollSeedanceTask(requestConfig, task, options) : pollOpenAIVideoTask(requestConfig, task, options);
}

async function createManagedCanvasVideoTask(
    config: AiConfig,
    selectedModel: string,
    model: string,
    prompt: string,
    references: ReferenceImage[],
    videoReferences: ReferenceVideo[],
    audioReferences: ReferenceAudio[],
    options?: RequestOptions,
): Promise<VideoGenerationTask> {
    const mediaInput = async (item: ReferenceVideo | ReferenceAudio, kind: "Video" | "Audio") => ({
        name: item.name,
        type: item.type,
        dataUrl: isPublicMediaUrl(item.url) || item.url.startsWith("asset://") ? item.url : await readFileAsDataUrl(await referenceMediaToFile(item, kind === "Video" ? "reference.mp4" : "reference.mp3", kind === "Video" ? "invalidReferenceVideo" : "invalidReferenceAudio", options)),
        durationSeconds: (item.durationMs || 0) / 1000,
    });
    const videos = await Promise.all(videoReferences.map((item) => mediaInput(item, "Video")));
    const audios = await Promise.all(audioReferences.map((item) => mediaInput(item, "Audio")));
    const images = await Promise.all(
        references.map(async (image, index) => ({
            name: image.name || `reference-${index + 1}.png`,
            type: image.type || "image/png",
            dataUrl: await imageToDataUrl(image),
            frame: videoMode(config, references.length, videoReferences.length, audioReferences.length) === "frames" ? (index === 0 ? "first" : index === 1 ? "last" : "") : "",
        })),
    );
    const run = await startCanvasRun(
        config,
        {
            mode: "video",
            model,
            prompt,
            images,
            referenceVideos: videos,
            referenceAudios: audios,
            duration: Number(config.videoSeconds) < 0 ? Math.max(1, ...videoReferences.map((item) => Math.ceil((item.durationMs || 0) / 1000))) : Number(normalizeVideoSeconds(config.videoSeconds)),
            size: videoOutputSize(config),
            resolution: normalizeVideoResolution(config.vquality, model),
            aspectRatio: normalizeVideoAspectRatio(config.videoAspectRatio),
            videoStandard: videoProfile(config).id,
            videoProtocol: config.videoTransport === "native" ? videoProfile(config).protocol : config.videoTransport === "relay" ? "relay" : "auto",
            videoMode: videoMode(config, references.length, videoReferences.length, audioReferences.length),
            generateAudio: videoProfile(config).audio ? boolConfig(config.videoGenerateAudio, true) : undefined,
            watermark: videoProfile(config).watermark ? boolConfig(config.videoWatermark, false) : undefined,
            negativePrompt: videoProfile(config).negative ? config.videoNegativePrompt || undefined : undefined,
            seed: ["ark", "veo", "relay"].includes(videoProfile(config).protocol) && config.videoSeed.trim() ? Number(config.videoSeed) : undefined,
            klingMode: config.videoKlingMode || "pro",
        },
        options?.signal,
        selectedModel,
    );
    await options?.onRunStarted?.(run.id);
    return { id: run.id, provider: "canvas", model };
}

async function createPluginVideoTask(config: AiConfig, model: string, script: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    if (!config.baseUrl.trim()) throw new Error(apiText("baseUrlRequired"));
    if (!config.apiKey.trim()) throw new Error(apiText("apiKeyRequired"));
    const refs = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const videos = await Promise.all((options?.videos || []).map((video) => referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((audio) => referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options)));
    const result = videoPluginResult(
        await runModelPlugin({
            capability: "video",
            script,
            config,
            prompt,
            images: refs,
            videos,
            audios,
            params: {
                seconds: normalizeVideoSeconds(config.videoSeconds),
                size: normalizeVideoSize(config.size),
                resolution: normalizeVideoResolution(config.vquality, model),
                ratio: normalizeVideoAspectRatio(config.videoAspectRatio),
                generateAudio: boolConfig(config.videoGenerateAudio, true),
                watermark: boolConfig(config.videoWatermark, false),
                mode: videoMode(config, refs.length, videos.length, audios.length),
                negativePrompt: config.videoNegativePrompt,
                seed: config.videoSeed,
                standard: videoProfile(config).id,
            },
            signal: options?.signal,
        }),
    );
    const id = nanoid();
    pluginVideoResults.set(id, result);
    return { id, provider: "plugin", model };
}

function videoPluginResult(result: unknown): VideoGenerationResult {
    if (result instanceof Blob) return { blob: result };
    if (typeof result === "string") return { url: result, mimeType: "video/mp4" };
    if (result && typeof result === "object") {
        const record = result as Record<string, unknown>;
        if (record.blob instanceof Blob) return { blob: record.blob };
        const url = [record.url, record.video_url, record.result_url].find((value) => typeof value === "string" && value) as string | undefined;
        if (url) return { url, mimeType: "video/mp4" };
    }
    throw new Error(apiText("scriptNoVideo"));
}

export async function storeGeneratedVideo(result: VideoGenerationResult): Promise<UploadedFile> {
    if (result.blob) return uploadMediaFile(result.blob, "video");
    if (result.url) {
        try {
            return await uploadMediaFile(result.url, "video");
        } catch {
            return { url: result.url, storageKey: "", bytes: 0, mimeType: result.mimeType || "video/mp4" };
        }
    }
    throw new Error(apiText("noPlayableVideo"));
}

async function createOpenAIVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    const body = new FormData();
    body.append("model", modelOptionName(model));
    body.append("prompt", prompt);
    body.append("seconds", normalizeVideoSeconds(config.videoSeconds));
    body.append("size", videoOutputSize(config));
    if (videoProfile(config).id !== "sora-2") {
        body.append("resolution_name", normalizeVideoResolution(config.vquality, model));
        body.append("preset", "normal");
        body.append("aspect_ratio", config.videoAspectRatio);
        body.append("video_mode", videoMode(config, references.length, options?.videos?.length, options?.audios?.length));
        if (videoProfile(config).audio) body.append("generate_audio", config.videoGenerateAudio);
        if (videoProfile(config).watermark) body.append("watermark", config.videoWatermark);
        if (videoProfile(config).protocol === "kling") body.append("mode", config.videoKlingMode || "pro");
        if (videoProfile(config).negative && config.videoNegativePrompt) body.append("negative_prompt", config.videoNegativePrompt);
        if (["ark", "veo", "relay"].includes(videoProfile(config).protocol) && config.videoSeed.trim()) body.append("seed", config.videoSeed);
    }
    const files = await Promise.all(references.map(async (image) => dataUrlToFile({ ...image, dataUrl: await imageToDataUrl(image) })));
    files.forEach((file) => body.append(videoProfile(config).id === "sora-2" ? "input_reference" : "input_reference[]", file));
    const videos = await Promise.all((options?.videos || []).map((item) => referenceMediaToFile(item, "reference.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((item) => referenceMediaToFile(item, "reference.mp3", "invalidReferenceAudio", options)));
    videos.forEach((file) => body.append("input_reference[]", file));
    audios.forEach((file) => body.append("input_reference[]", file));
    try {
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), body, { headers: aiHeaders(config), signal: options?.signal })).data);
        if (!created.id) throw new Error(apiText("noVideoTaskId"));
        return { id: created.id, provider: "openai", model };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("videoTaskCreateFailed")));
    }
}

async function pollOpenAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const video = unwrapVideoResponse((await axios.get<ApiVideoResponse>(aiApiUrl(config, `/videos/${task.id}`), { headers: aiHeaders(config), signal: options?.signal })).data);
        const url = videoResultUrl(video);
        if (url) return { status: "completed", result: await videoResultFromUrl(url, options) };
        if (video.status === "completed") {
            const content = await axios.get<Blob>(aiApiUrl(config, `/videos/${task.id}/content`), { headers: aiHeaders(config), responseType: "blob", signal: options?.signal });
            await assertVideoBlob(content.data);
            return { status: "completed", result: { blob: content.data } };
        }
        if (video.status === "failed" || video.status === "cancelled") return { status: "failed", error: readApiErrorMessage(video.error?.message) || apiText("videoGenerationFailed") };
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("videoTaskQueryFailed")));
    }
}

async function createSeedanceTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], videoReferences: ReferenceVideo[], audioReferences: ReferenceAudio[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const content = await buildSeedanceContent(config, prompt, references, videoReferences, audioReferences);
    if (!content.length) throw new Error(apiText("videoPromptRequired"));
    const payload = {
        model: modelOptionName(model),
        content,
        ratio: config.videoAspectRatio,
        resolution: normalizeVideoResolution(config.vquality, model),
        duration: Number(config.videoSeconds),
        generate_audio: videoProfile(config).audio ? boolConfig(config.videoGenerateAudio, true) : undefined,
        watermark: boolConfig(config.videoWatermark, false),
        seed: config.videoSeed.trim() ? Number(config.videoSeed) : undefined,
        omni_reference_task_type: videoProfile(config).id === "seedance-2.5" && ["reference", "edit", "extend"].includes(config.videoMode) ? config.videoMode : undefined,
    };

    try {
        const created = unwrapSeedanceTask((await axios.post<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        if (!created.id) throw new Error(apiText("seedanceNoTaskId"));
        return { id: created.id, provider: "seedance", model };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("seedanceTaskCreateFailed")));
    }
}

async function pollSeedanceTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapSeedanceTask((await axios.get<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config, task.id), { headers: aiHeaders(config), signal: options?.signal })).data);
        const url = videoResultUrl(state);
        if (url) return { status: "completed", result: await videoResultFromUrl(url, options) };
        if (state.status === "succeeded" || state.status === "completed") return { status: "failed", error: apiText("seedanceNoVideoUrl") };
        if (state.status === "failed" || state.status === "cancelled" || state.status === "expired") return { status: "failed", error: readApiErrorMessage(state.error?.message) || apiText(state.status === "expired" ? "seedanceVideoTimeout" : "seedanceVideoFailed") };
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("seedanceTaskQueryFailed")));
    }
}

function seedanceApiUrl(config: AiConfig, taskId?: string) {
    return nativeVideoUrl(config, `/api/v3/contents/generations/tasks${taskId ? `/${encodeURIComponent(taskId)}` : ""}`);
}

function nativeVideoUrl(config: AiConfig, path: string) {
    const base = config.baseUrl.trim().replace(/\/(?:v1beta|v1|v2|api\/v3)\/?$/, "").replace(/\/+$/, "");
    return withLocalProxy(`${base}${path}`);
}

async function createNativeVideoTask(config: AiConfig, model: string, prompt: string, images: ReferenceImage[], videos: ReferenceVideo[], audios: ReferenceAudio[], protocol: string, options?: RequestOptions): Promise<VideoGenerationTask> {
    const mode = videoMode(config, images.length, videos.length, audios.length);
    const imageURLs = await Promise.all(images.map((image) => resolveSeedanceImageUrl(config, image)));
    const videoURLs = await Promise.all(videos.map(resolveSeedanceVideoUrl));
    const audioURLs = await Promise.all(audios.map(resolveSeedanceAudioUrl));
    const content = [
        { type: "text", text: prompt },
        ...imageURLs.map((url, i) => ({ type: "image_url", image_url: { url }, role: mode === "frames" && i < 2 ? (i ? "last_frame" : "first_frame") : "reference_image" })),
        ...videoURLs.map((url) => ({ type: "video_url", video_url: { url }, role: "reference_video" })),
        ...audioURLs.map((url) => ({ type: "audio_url", audio_url: { url }, role: "reference_audio" })),
    ];
    let path = "", pollPath = "";
    let body: Record<string, unknown> = {};
    if (protocol === "minimax-h3") {
        path = "/v2/video_generation"; pollPath = "/v2/query/video_generation/";
        body = { model: modelOptionName(model), content, duration: Number(config.videoSeconds), resolution: config.vquality === "2K" ? "2K" : `${config.vquality}P`, ...(mode === "frames" && images.length ? {} : { ratio: config.videoAspectRatio }) };
    } else if (protocol === "minimax") {
        if (videos.length || audios.length || images.length > 2 || (images.length > 1 && mode !== "frames")) throw new Error("Hailuo 原生接口仅支持首帧 / 首尾帧；全能参考请使用 MiniMax H3 或兼容网关。");
        path = "/v1/video_generation"; pollPath = "/v1/query/video_generation?task_id=";
        body = { model: modelOptionName(model), prompt, duration: Number(config.videoSeconds), resolution: `${config.vquality}P`, first_frame_image: imageURLs[0], last_frame_image: imageURLs[1] };
    } else if (protocol === "kling-v3") {
        if (audios.length || videos.length > 1) throw new Error("可灵原生接口不支持当前参考组合，请调整素材或选择兼容网关。");
        const profile = videoProfile(config), o1 = profile.id === "kling-o1", turbo = profile.id === "kling-3-turbo";
        const omni = o1 || profile.id === "kling-3-omni";
        if (!omni && (videos.length || images.length > 2)) throw new Error("请为多图 / 视频参考选择可灵 3.0 Omni 标准。");
        if (mode === "frames" && images.length > 2) throw new Error("首尾帧模式最多两张图，请切换全能参考。");
        if (turbo && images.length > 1) throw new Error("可灵 3.0 Turbo 仅支持首帧图，素材未截断。");
        path = o1 ? "/omni-video/kling-o1" : omni ? "/omni-video/kling-3.0-omni" : `${images.length ? "/image-to-video" : "/text-to-video"}/kling-3.0${turbo ? "-turbo" : ""}`;
        pollPath = "/tasks?task_ids=";
        const settings = { resolution: normalizeVideoResolution(config.vquality, model), duration: Number(config.videoSeconds), ...(omni || !images.length ? { aspect_ratio: config.videoAspectRatio } : {}), ...(!turbo ? { audio: o1 ? videos.length && boolConfig(config.videoGenerateAudio, true) ? "original" : "off" : videos.length ? "off" : boolConfig(config.videoGenerateAudio, true) ? "native" : "off" } : {}), ...(!o1 && !turbo ? { multi_shot: mode !== "edit" } : {}) };
        if (imageURLs.some((value) => !isPublicMediaUrl(value) && !value.startsWith("data:image/")) || videoURLs.some((value) => !isPublicMediaUrl(value))) throw new Error("可灵图片需为 HTTP(S) / Base64，视频需为公网 HTTP(S)；本地视频请使用已配置对象存储的托管渠道。");
        if (mode === "edit" && images.length && content.some((item) => item.role === "first_frame" || item.role === "last_frame")) throw new Error("可灵视频编辑不能混用首尾帧。");
        body = { settings, options: { watermark_info: { enabled: boolConfig(config.videoWatermark, false) } }, ...(omni || images.length ? { contents: [{ type: "prompt", text: prompt }, ...imageURLs.map((url, i) => ({ type: !omni || mode === "frames" ? (i ? "last_frame" : "first_frame") : "refer_image", url: url.startsWith("data:image/") ? url.split(";base64,")[1] : url, ...(omni ? { id: `image_${i + 1}` } : {}) })), ...videoURLs.map((url) => ({ type: mode === "edit" ? "base_video" : "feature_video", url, id: "video_1" }))] } : { prompt }) };
    } else if (protocol === "kling") {
        if (videos.length || audios.length || images.length > 2 || (images.length > 1 && mode !== "frames")) throw new Error("可灵旧版原生接口只支持首尾帧；请切换对应 Omni 标准或兼容网关。");
        path = images.length ? "/v1/videos/image2video" : "/v1/videos/text2video"; pollPath = `${path}/`;
        body = { model_name: modelOptionName(model), prompt, negative_prompt: config.videoNegativePrompt || undefined, duration: config.videoSeconds, mode: config.videoKlingMode, aspect_ratio: config.videoAspectRatio, image: imageURLs[0], image_tail: imageURLs[1], ...(videoProfile(config).audio ? { sound: boolConfig(config.videoGenerateAudio, true) ? "on" : "off" } : {}) };
    } else if (protocol === "omni") {
        if (audios.length) throw new Error("Gemini Omni 当前原生 API 不接收音频参考，请选择兼容网关或其他标准。");
        path = "/v1beta/interactions"; pollPath = `${path}/`;
        const media = await Promise.all([...images.map(async (image) => ({ type: "image", ...omniInline(await imageToDataUrl(image)) })), ...videos.map(async (video) => ({ type: "video", ...omniInline(await readFileAsDataUrl(await referenceMediaToFile(video, "reference.mp4", "invalidReferenceVideo", options))) }))]);
        const tags = mode === "frames" ? images.map((_, i) => i < 2 ? `${i ? "<LAST_FRAME>" : "<FIRST_FRAME>"}@Image${i + 1}` : "").filter(Boolean).join(" ") : "";
        body = { model: modelOptionName(model), input: [...media, { type: "text", text: `${tags ? `[# Sources ${tags}] ` : ""}${prompt}` }], response_format: { type: "video", aspect_ratio: config.videoAspectRatio, resolution: normalizeVideoResolution(config.vquality, model), delivery: "uri" }, ...(mode === "edit" || mode === "extend" ? { generation_config: { video_config: { task: mode } } } : {}) };
    }
    try {
        const raw = (await axios.post(nativeVideoUrl(config, path), body, { headers: protocol === "omni" ? geminiVideoHeaders(config) : aiHeaders(config, "application/json"), signal: options?.signal })).data;
        const value = nativeVideoResponse(raw);
        const id = value.task_id || value.id;
        if (!id) throw new Error(apiText("noVideoTaskId"));
        const result = nativeVideoResult(value);
        if (result && !(protocol === "omni" && result.url)) pluginVideoResults.set(String(id), result);
        return { id: String(id), provider: protocol as VideoGenerationTask["provider"], model, path: pollPath };
    } catch (error) { throw new Error(readAxiosError(error, apiText("videoTaskCreateFailed"))); }
}

function omniInline(dataUrl: string) {
    const value = parseDataUrlInline(dataUrl);
    return { data: value.bytesBase64Encoded, mime_type: value.mimeType };
}

function nativeVideoResponse(raw: any): any {
    if (raw?.base_resp?.status_code) throw new Error(raw.base_resp.status_msg || "MiniMax 请求失败");
    if (raw?.code && raw.code !== "0") throw new Error(raw.message || raw.msg || "视频请求失败");
    const value = Array.isArray(raw?.data) ? raw.data[0] : raw?.data || raw;
    if (!value) throw new Error("视频接口未返回任务");
    return value.task || value;
}

function nativeVideoResult(value: any): VideoGenerationResult | undefined {
    const part = value.steps?.filter((step: any) => step.type === "model_output").flatMap((step: any) => step.content || []).find((item: any) => item.type === "video");
    if (part?.data) return { blob: dataUrlToBlob(`data:${part.mime_type || "video/mp4"};base64,${part.data}`) };
    const url = part?.uri || value.content?.url || value.content?.video_url || value.outputs?.find((o: any) => o.type === "video")?.url || value.task_result?.videos?.[0]?.url;
    if (url) return { url, mimeType: "video/mp4" };
}

async function pollNativeVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    const cached = pluginVideoResults.get(task.id);
    if (cached) return { status: "completed", result: cached };
    const paths: Record<string, string> = { minimax: "/v1/query/video_generation?task_id=", "minimax-h3": "/v2/query/video_generation/", "kling-v3": "/tasks?task_ids=", omni: "/v1beta/interactions/" };
    if (!task.path && task.provider === "kling") throw new Error("可灵任务缺少查询路径，请重新生成。");
    const headers = task.provider === "omni" ? geminiVideoHeaders(config) : aiHeaders(config, "application/json");
    const value = nativeVideoResponse((await axios.get(nativeVideoUrl(config, `${task.path || paths[task.provider]}${encodeURIComponent(task.id)}`), { headers, signal: options?.signal })).data);
    const status = String(value.status || value.task_status || "").toLowerCase();
    if (["failed", "error", "cancelled", "canceled", "expired"].includes(status)) return { status: "failed", error: value.message || value.task_status_msg || value.base_resp?.status_msg || apiText("videoGenerationFailed") };
    let result = nativeVideoResult(value);
    if (!result && value.file_id && task.provider === "minimax") {
        const file = nativeVideoResponse((await axios.get(nativeVideoUrl(config, `/v1/files/retrieve?file_id=${encodeURIComponent(value.file_id)}`), { headers, signal: options?.signal })).data);
        if (file.file?.download_url) result = { url: file.file.download_url };
    }
    if (result?.url && task.provider === "omni") {
        const target = new URL(result.url);
        if (target.protocol !== "https:" || target.hostname !== "generativelanguage.googleapis.com") throw new Error("Gemini 返回了非官方下载地址");
        if (target.pathname.startsWith("/v1beta/files/")) {
            const fileURL = new URL(target.href);
            fileURL.pathname = fileURL.pathname.replace(/:download$/, ""); fileURL.search = "";
            const file = (await axios.get(withLocalProxy(fileURL.href), { headers, signal: options?.signal })).data;
            if (file.state === "PROCESSING") return { status: "pending" };
            if (file.state !== "ACTIVE") return { status: "failed", error: "Gemini 视频文件处理失败" };
        }
        const response = await axios.get<Blob>(withLocalProxy(target.href), { headers, responseType: "blob", signal: options?.signal });
        result = { blob: response.data };
    }
    if (result) return { status: "completed", result };
    if (["succeed", "succeeded", "success", "completed"].includes(status)) return { status: "failed", error: apiText("noPlayableVideo") };
    return { status: "pending" };
}

async function buildSeedanceContent(config: AiConfig, prompt: string, references: ReferenceImage[], videoReferences: ReferenceVideo[], audioReferences: ReferenceAudio[]) {
    const content: Array<Record<string, unknown>> = [];
    const text = buildSeedancePromptText(prompt, references, videoReferences, audioReferences);
    if (text) content.push({ type: "text", text });
    const mode = videoMode(config, references.length, videoReferences.length, audioReferences.length);
    for (const [index, image] of references.entries()) {
        content.push({ type: "image_url", image_url: { url: await resolveSeedanceImageUrl(config, image) }, role: mode === "frames" && index < 2 ? (index === 0 ? "first_frame" : "last_frame") : "reference_image" });
    }
    for (const video of videoReferences) {
        content.push({ type: "video_url", video_url: { url: await resolveSeedanceVideoUrl(video) }, role: "reference_video" });
    }
    for (const audio of audioReferences) {
        content.push({ type: "audio_url", audio_url: { url: await resolveSeedanceAudioUrl(audio) }, role: "reference_audio" });
    }
    return content;
}

async function resolveSeedanceImageUrl(config: AiConfig, image: ReferenceImage) {
    const directUrl = image.url || image.dataUrl;
    if (isPublicMediaUrl(directUrl) || directUrl.startsWith("asset://")) return directUrl;
    const dataUrl = await imageToDataUrl(image);
    if (!dataUrl) throw new Error(apiText("referenceImageReadFailed"));
    return dataUrl;
}

async function resolveSeedanceVideoUrl(video: ReferenceVideo) {
    if (video.url.startsWith("data:")) return video.url;
    if (isPublicMediaUrl(video.url) || video.url.startsWith("asset://")) return video.url;
    let blob: Blob | null = null;
    if (video.storageKey) blob = await getMediaBlob(video.storageKey);
    if (!blob && video.url?.startsWith("blob:")) blob = await (await fetch(video.url)).blob();
    if (!blob) throw new Error(apiText("invalidReferenceVideo"));
    return blobToDataUrl(blob);
}

async function resolveSeedanceAudioUrl(audio: ReferenceAudio) {
    if (audio.url.startsWith("data:")) return audio.url;
    if (isPublicMediaUrl(audio.url) || audio.url.startsWith("asset://")) return audio.url;
    let blob: Blob | null = null;
    if (audio.storageKey) blob = await getMediaBlob(audio.storageKey);
    if (!blob && audio.url?.startsWith("blob:")) blob = await (await fetch(audio.url)).blob();
    if (!blob) throw new Error(apiText("invalidReferenceAudio"));
    return blobToDataUrl(blob);
}

async function videoResultFromUrl(url: string, options?: RequestOptions): Promise<VideoGenerationResult> {
    try {
        const response = await axios.get<Blob>(url, { responseType: "blob", signal: options?.signal });
        await assertVideoBlob(response.data);
        return { blob: response.data };
    } catch (error) {
        if (axios.isCancel(error) || options?.signal?.aborted) throw error;
        return { url, mimeType: "video/mp4" };
    }
}

async function createGeminiVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    const images = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const videos = await Promise.all((options?.videos || []).map((video) => referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((audio) => referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options)));
    const mode = videoMode(config, images.length, videos.length, audios.length);
    const instance: Record<string, unknown> = { prompt };
    if (mode === "frames") {
        if (images[0]) instance.image = parseDataUrlInline(images[0]);
        if (images[1]) instance.lastFrame = parseDataUrlInline(images[1]);
        if (images.length > 2) throw new Error("Veo 首尾帧模式最多两张图，请切换全能参考。");
    } else {
        instance.referenceImages = images.map((dataUrl) => ({ image: parseDataUrlInline(dataUrl), referenceType: "asset" }));
    }
    if (videos[0]) instance.video = await fileToGeminiInline(videos[0]);
    if (audios.length || videos.length > 1 || (videos.length && (mode !== "extend" || images.length))) throw new Error("Veo 原生只支持单视频延长，不支持混用图片、参考音频或多视频；请切换接口标准或使用兼容网关。");
    try {
        const created = unwrapEnvelope((await axios.post<ApiEnvelope<GeminiVideoOperation>>(geminiVideoUrl(config, model, "predictLongRunning"), {
            instances: [instance],
            parameters: {
                aspectRatio: normalizeVideoAspectRatio(config.videoAspectRatio),
                durationSeconds: Number(normalizeVideoSeconds(config.videoSeconds)) || 8,
                resolution: normalizeVideoResolution(config.vquality, model),
                negativePrompt: config.videoNegativePrompt || undefined,
                seed: config.videoSeed.trim() ? Number(config.videoSeed) : undefined,
            },
        }, { headers: geminiVideoHeaders(config), signal: options?.signal })).data, apiText("noVideoTask"));
        if (!created.name) throw new Error(apiText("noVideoTaskId"));
        return { id: created.name, provider: "gemini", model };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("videoTaskCreateFailed")));
    }
}

async function pollGeminiVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapEnvelope((await axios.get<ApiEnvelope<GeminiVideoOperation>>(geminiOperationUrl(config, task.id), { headers: geminiVideoHeaders(config), signal: options?.signal })).data, apiText("videoTaskQueryFailed"));
        if (state.error) return { status: "failed", error: readApiErrorMessage(state.error.message) || apiText("videoGenerationFailed") };
        if (!state.done) return { status: "pending" };
        const uri = state.response?.generateVideoResponse?.generatedSamples?.[0]?.video?.uri;
        if (!uri) return { status: "failed", error: apiText("noPlayableVideo") };
        const target = new URL(uri);
        if (target.protocol !== "https:" || target.hostname !== "generativelanguage.googleapis.com") throw new Error("Veo 返回了非官方下载地址");
        const content = await axios.get<Blob>(withLocalProxy(target.href), { headers: geminiVideoHeaders(config), responseType: "blob", signal: options?.signal });
        await assertVideoBlob(content.data);
        return { status: "completed", result: { blob: content.data } };
    } catch (error) {
        throw new Error(readAxiosError(error, apiText("videoTaskQueryFailed")));
    }
}

function geminiVideoBaseUrl(config: Pick<AiConfig, "baseUrl">) {
    const normalizedBaseUrl = config.baseUrl.trim().replace(/\/+$/, "");
    const lowerBaseUrl = normalizedBaseUrl.toLowerCase();
    return lowerBaseUrl.endsWith("/v1") || lowerBaseUrl.endsWith("/v1beta") ? normalizedBaseUrl : `${normalizedBaseUrl}/v1beta`;
}

function geminiVideoUrl(config: Pick<AiConfig, "baseUrl">, model: string, action: string) {
    return withLocalProxy(`${geminiVideoBaseUrl(config)}/models/${encodeURIComponent(modelOptionName(model).replace(/^models\//, ""))}:${action}`);
}

function geminiOperationUrl(config: Pick<AiConfig, "baseUrl">, name: string) {
    return withLocalProxy(`${geminiVideoBaseUrl(config)}/${name.replace(/^\//, "")}`);
}

function geminiVideoHeaders(config: Pick<AiConfig, "apiKey">) {
    return { "x-goog-api-key": config.apiKey, "Content-Type": "application/json" };
}

function parseDataUrlInline(dataUrl: string, fallbackType = "image/png"): GeminiInlineData {
    const match = dataUrl.match(/^data:([^;]+);base64,(.*)$/);
    return { bytesBase64Encoded: match?.[2] || "", mimeType: match?.[1] || fallbackType };
}

async function fileToGeminiInline(file: File): Promise<GeminiInlineData> {
    return parseDataUrlInline(await readFileAsDataUrl(file), file.type || "application/octet-stream");
}

async function referenceMediaToFile(item: { name: string; type?: string; url?: string; storageKey?: string }, fallbackName: string, errorKey: "invalidReferenceVideo" | "invalidReferenceAudio", options?: RequestOptions) {
    let blob = item.storageKey ? await getMediaBlob(item.storageKey) : null;
    if (!blob) {
        const url = item.storageKey ? await resolveMediaUrl(item.storageKey, item.url || "") : item.url || "";
        if (!url) throw new Error(apiText(errorKey));
        try {
            blob = url.startsWith("data:") ? dataUrlToBlob(url) : await (await fetch(withLocalProxy(url), { signal: options?.signal })).blob();
        } catch (error) {
            if (error instanceof DOMException && error.name === "AbortError") throw error;
            throw new Error(apiText(errorKey));
        }
    }
    if (!blob.size) throw new Error(apiText(errorKey));
    return new File([blob], item.name || fallbackName, { type: item.type || blob.type || "application/octet-stream" });
}

function assertVideoConfig(config: AiConfig, model: string) {
    if (!model) throw new Error(apiText("videoModelRequired"));
    if (!config.baseUrl.trim()) throw new Error(apiText("baseUrlRequired"));
    if (!config.apiKey.trim()) throw new Error(apiText("apiKeyRequired"));
}

function normalizeVideoSeconds(value: string) {
    const seconds = Math.floor(Number(value) || 6);
    return String(seconds);
}

function normalizeVideoSize(value: string) {
    if (value === "auto") return null;
    const size = value || "1280x720";
    if (/^\d+x\d+$/.test(size)) return size;
    return ["9:16", "2:3", "3:4"].includes(size) ? "720x1280" : "1280x720";
}

function videoOutputSize(config: AiConfig) {
    if (videoProfile(config).id === "sora-2" && config.vquality === "1024") return config.videoAspectRatio === "9:16" ? "1024x1792" : "1792x1024";
    const height = /4k/i.test(config.vquality) ? 2160 : /2k/i.test(config.vquality) ? 1440 : Number(config.vquality.replace(/p$/i, "")) || 720;
    const [w, h] = config.videoAspectRatio.split(":").map(Number);
    if (!w || !h) return `${Math.round(height * 16 / 9 / 2) * 2}x${height}`;
    return w >= h ? `${Math.round(height * w / h / 2) * 2}x${height}` : `${height}x${Math.round(height * h / w / 2) * 2}`;
}

function normalizeVideoResolution(value: string, model: string) {
    if (isXiaowenVideoModel(model)) return normalizeXiaowenResolution(value);
    if (/^[24]k$/i.test(value)) return value.toLowerCase();
    if (value === "low") return "480p";
    if (value === "auto" || value === "high" || value === "medium") return "720p";
    const resolution = value.replace(/p$/i, "") || "720";
    return `${resolution}p`;
}

function unwrapVideoResponse(payload: ApiVideoResponse) {
    return unwrapEnvelope(payload, apiText("noVideoTask"));
}

function unwrapSeedanceTask(payload: ApiEnvelope<SeedanceTask>) {
    return unwrapEnvelope(payload, apiText("seedanceNoTask"));
}

function unwrapEnvelope<T>(payload: ApiEnvelope<T>, emptyMessage: string): T {
    if (!payload) throw new Error(emptyMessage);
    if (typeof payload === "object" && "code" in payload && payload.code !== undefined) {
        if (payload.code !== 0 && payload.code !== "0") throw new Error(readApiErrorMessage(payload) || apiText("requestFailed"));
        if (!payload.data) throw new Error(emptyMessage);
        return payload.data;
    }
    return payload as T;
}

function videoResultUrl(payload: VideoResponse | SeedanceTask) {
    return [payload.video_url, payload.result_url, payload.url, payload.content?.video_url, payload.content?.url].find((url) => typeof url === "string" && (isPublicMediaUrl(url) || /\.mp4(\?|#|$)/i.test(url)));
}

function readApiErrorMessage(value: unknown): string {
    if (!value) return "";
    if (typeof value === "string") {
        try {
            const parsed = JSON.parse(value);
            const inner = readApiErrorMessage(parsed) || value;
            if (inner === value && typeof parsed === "object" && Object.keys(parsed).length === 0) return "";
            return inner;
        } catch {
            if (/<[a-z][\s\S]*>/i.test(value)) return apiText("htmlError", { preview: `${value.slice(0, 80)}...` });
            return value;
        }
    }
    if (typeof value !== "object") return "";
    const payload = value as { msg?: unknown; message?: unknown; error?: unknown; detail?: unknown };
    // error may be a string or an object containing a message.
    const errorMsg =
        typeof payload.error === "string"
            ? payload.error
            : (payload.error as { message?: unknown })?.message;
    return (
        readApiErrorMessage(payload.msg) ||
        readApiErrorMessage(payload.message) ||
        readApiErrorMessage(errorMsg) ||
        readApiErrorMessage(payload.detail) ||
        ""
    );
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isCancel(error)) return apiText("requestCanceled");
    if (axios.isAxiosError<{ error?: { message?: string }; msg?: string; message?: string; code?: number | string }>(error)) {
        const responseData = error.response?.data;
        return readApiErrorMessage(responseData) || statusMessage(error.response?.status, fallback);
    }
    if (error instanceof DOMException && error.name === "AbortError") return apiText("requestCanceled");
    return error instanceof Error ? readApiErrorMessage(error.message) || error.message : fallback;
}

function statusMessage(status: number | undefined, fallback: string) {
    if (status === 401 || status === 403) return apiText("authenticationFailed");
    if (status === 429) return apiText("rateLimited");
    return status ? `${fallback}（${status}）` : fallback;
}

async function assertVideoBlob(blob: Blob) {
    if (!blob.type.includes("json")) return;
    let payload: { code?: number; msg?: string; error?: { message?: string } };
    try {
        payload = JSON.parse(await blob.text()) as { code?: number; msg?: string; error?: { message?: string } };
    } catch {
        return;
    }
    if (typeof payload.code === "number" && payload.code !== 0) throw new Error(readApiErrorMessage(payload) || apiText("videoDownloadFailed"));
    if (payload.error?.message) throw new Error(readApiErrorMessage(payload.error.message) || payload.error.message);
}

function isPublicMediaUrl(value: string) {
    return /^https?:\/\//i.test(value || "");
}

function delay(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        if (signal?.aborted) {
            reject(new DOMException("Aborted", "AbortError"));
            return;
        }
        const timer = setTimeout(resolve, ms);
        signal?.addEventListener(
            "abort",
            () => {
                clearTimeout(timer);
                reject(new DOMException("Aborted", "AbortError"));
            },
            { once: true },
        );
    });
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error(apiText("localAssetReadFailed")));
        reader.readAsDataURL(blob);
    });
}
