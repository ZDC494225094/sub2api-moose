import type { ReferenceVideo } from "@/types/media";

type TranscodeOptions = { signal?: AbortSignal; onProgress?: (percent: number) => void };

// Keep only the latest small result for retries; originals remain in media storage unchanged.
let recent: { key: string; blob: Blob } | undefined;

export function isWebMReference(video: ReferenceVideo) {
    return /^video\/webm(?:;|$)/i.test(video.type) || /\.webm$/i.test(video.name) || /\.webm(?:[?#]|$)/i.test(video.url) || /^data:video\/webm[;,]/i.test(video.url);
}

export async function transcodeReferenceToMP4(blob: Blob, key: string | undefined, options: TranscodeOptions = {}) {
    options.signal?.throwIfAborted();
    if (key && recent?.key === key) return recent.blob;
    const worker = new Worker(new URL("../lib/canvas/video-mp4.worker.ts", import.meta.url), { type: "module" });
    let abort: (() => void) | undefined;
    try {
        const result = await new Promise<Blob>((resolve, reject) => {
            abort = () => reject(new DOMException("已取消", "AbortError"));
            options.signal?.addEventListener("abort", abort, { once: true });
            worker.onerror = () => reject(new Error("MP4 转码 Worker 启动失败，请检查浏览器安全策略。"));
            worker.onmessageerror = () => reject(new Error("MP4 转码结果读取失败。"));
            worker.onmessage = ({ data }) => {
                if (data.type === "progress") options.onProgress?.(data.percent);
                else if (data.type === "complete") resolve(data.blob);
                else if (data.type === "error") reject(new Error(data.message));
            };
            worker.postMessage({ blob });
        });
        options.signal?.throwIfAborted();
        recent = key && result.size <= 128 * 1024 * 1024 ? { key, blob: result } : undefined;
        return result;
    } finally {
        if (abort) options.signal?.removeEventListener("abort", abort);
        worker.terminate();
    }
}
