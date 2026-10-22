import { env, pipeline, RawImage } from "@huggingface/transformers";
import { ALL_FORMATS, BlobSource, BufferTarget, CanvasSource, Input, Mp4OutputFormat, Output, QUALITY_HIGH, VideoSampleSink, WebMOutputFormat, canEncodeVideo } from "mediabunny";

self.onmessage = async ({ data }: MessageEvent<{ blob: Blob; mode: "auto" | "cpu"; samplingFps: number; wasm: { mjs: string; wasm: string } }>) => {
    const input = new Input({ source: new BlobSource(data.blob), formats: ALL_FORMATS });
    let output: Output | undefined;
    let estimator: Awaited<ReturnType<typeof createEstimator>> | undefined;
    const progress = (label: string, percent?: number, stats?: { backend: string; frames: number; inferred: number; fps: number; elapsed: number; remaining?: number }) => self.postMessage({ type: "progress", label, percent, stats });
    try {
        if (typeof VideoEncoder === "undefined" || typeof VideoDecoder === "undefined") throw new Error("当前浏览器不支持 WebCodecs，请使用新版 Chrome 或 Edge，并通过 HTTPS 或 localhost 打开画布。");
        const track = await input.getPrimaryVideoTrack();
        if (!track || !(await track.canDecode())) throw new Error("浏览器无法解码这个视频，请先将源视频转为 H.264 MP4 或 VP9 WebM。");
        const width = track.displayWidth;
        const height = track.displayHeight;
        const codec = await canEncodeVideo("avc", { width, height }) ? "avc" : "vp9";
        if (codec === "vp9" && !(await canEncodeVideo("vp9", { width, height }))) throw new Error("当前浏览器不支持此尺寸的 H.264 或 VP9 视频编码。");
        const duration = await track.computeDuration();
        progress("加载模型…");
        env.allowLocalModels = false;
        env.backends.onnx.wasm!.wasmPaths = data.wasm;
        env.backends.onnx.wasm!.numThreads = self.crossOriginIsolated ? Math.min(4, navigator.hardwareConcurrency || 1) : 1;
        env.backends.onnx.wasm!.proxy = false;
        let backend = "CPU";
        if (data.mode !== "cpu") {
            try {
                const gpu = (navigator as unknown as { gpu?: { requestAdapter(): Promise<{ features: { has(name: string): boolean } } | null> } }).gpu;
                const adapter = await gpu?.requestAdapter();
                if (adapter) {
                    progress("加载 GPU 模型…");
                    estimator = await createEstimator("webgpu", adapter.features.has("shader-f16") ? "fp16" : "fp32");
                    backend = "GPU (WebGPU)";
                }
            } catch {
                progress("切换至 CPU…");
            }
        }
        estimator ??= await createEstimator("wasm", "q8");
        progress(`${backend} · 准备转换`, 0);
        const sourceCanvas = new OffscreenCanvas(width, height);
        const depthCanvas = new OffscreenCanvas(width, height);
        const sourceContext = sourceCanvas.getContext("2d")!;
        const depthContext = depthCanvas.getContext("2d")!;
        const target = new BufferTarget();
        output = new Output({ format: codec === "avc" ? new Mp4OutputFormat() : new WebMOutputFormat(), target });
        const source = new CanvasSource(depthCanvas, { codec, bitrate: QUALITY_HIGH });
        output.addVideoTrack(source);
        await output.start();
        let count = 0;
        let inferred = 0;
        let lastBucket = -1;
        let firstTimestamp: number | undefined;
        const startedAt = performance.now();
        for await (const sample of new VideoSampleSink(track).samples()) {
            try {
                firstTimestamp ??= sample.timestamp;
                const timestamp = sample.timestamp - firstTimestamp;
                const bucket = data.samplingFps > 0 ? Math.floor((timestamp + 0.000001) * data.samplingFps) : count;
                // Infer the first frame in each time bucket; reuse its depth for other source frames.
                if (bucket !== lastBucket) {
                    sample.draw(sourceContext, 0, 0, width, height);
                    const result = await estimator(RawImage.fromCanvas(sourceCanvas));
                    const frame = Array.isArray(result) ? result[0] : result;
                    try {
                        depthContext.drawImage(frame.depth.toCanvas(), 0, 0, width, height);
                    } finally {
                        frame.predicted_depth.dispose();
                    }
                    lastBucket = bucket;
                    inferred += 1;
                }
                await source.add(timestamp, sample.duration);
                count += 1;
                const elapsed = (performance.now() - startedAt) / 1000;
                const processed = timestamp + sample.duration;
                const total = duration - firstTimestamp;
                // Wait for several inference samples so startup and held frames do not dominate the estimate.
                const remaining = inferred >= 3 && processed > 0 && total > 0 ? Math.max(0, Math.ceil((total - processed) * elapsed / processed)) : undefined;
                progress("正在生成深度视频", total > 0 ? Math.min(99, Math.round((processed / total) * 100)) : undefined, { backend, frames: count, inferred, fps: count / Math.max(elapsed, 0.001), elapsed, remaining });
            } finally {
                sample.close();
            }
        }
        if (!count) throw new Error("源视频没有可解码的视频帧。");
        progress("正在封装深度视频", 99);
        await output.finalize();
        if (!target.buffer) throw new Error("深度视频封装失败。");
        self.postMessage({ type: "complete", blob: new Blob([target.buffer], { type: codec === "avc" ? "video/mp4" : "video/webm" }) });
    } catch (error) {
        self.postMessage({ type: "error", message: error instanceof Error ? error.message : "深度视频转换失败" });
    } finally {
        if (output && output.state !== "finalized") await output.cancel().catch(() => {});
        await estimator?.dispose();
        input.dispose();
    }
};

function createEstimator(device: "webgpu" | "wasm", dtype: "fp16" | "fp32" | "q8") {
    return pipeline("depth-estimation", "onnx-community/depth-anything-v2-small", { device, dtype });
}
