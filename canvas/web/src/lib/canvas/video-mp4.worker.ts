import { ALL_FORMATS, BlobSource, BufferTarget, Conversion, Input, Mp4OutputFormat, Output, QUALITY_HIGH } from "mediabunny";

self.onmessage = async ({ data }: MessageEvent<{ blob: Blob }>) => {
    const input = new Input({ source: new BlobSource(data.blob), formats: ALL_FORMATS });
    const target = new BufferTarget();
    const output = new Output({ format: new Mp4OutputFormat(), target });
    try {
        if (typeof VideoEncoder === "undefined" || typeof VideoDecoder === "undefined") throw new Error("当前浏览器不支持视频转码，请使用新版 Chrome / Edge，通过 HTTPS 或 localhost 打开画布，或先将参考视频转为 H.264 MP4。");
        const track = await input.getPrimaryVideoTrack();
        if (!track) throw new Error("参考视频没有可读取的视频轨道。");
        // H.264 requires even dimensions. At most one pixel of padding preserves the full frame.
        const conversion = await Conversion.init({
            input,
            output,
            video: { codec: "avc", quality: QUALITY_HIGH, width: Math.ceil(track.displayWidth / 2) * 2, height: Math.ceil(track.displayHeight / 2) * 2, fit: "contain", allowTransformationMetadata: false },
            audio: { codec: "aac", quality: QUALITY_HIGH },
            showWarnings: false,
        });
        if (!conversion.isValid || !conversion.utilizedTracks.some((item) => item.isVideoTrack()) || conversion.discardedTracks.some((item) => item.track.isVideoTrack() || item.track.isAudioTrack())) throw new Error("浏览器无法完整转为 H.264 / AAC MP4，请先在外部转码；不会丢弃画面或音频后继续提交。");
        let lastPercent = -1;
        conversion.onProgress = (value) => {
            const percent = Math.min(99, Math.floor(value * 100));
            if (percent !== lastPercent) self.postMessage({ type: "progress", percent });
            lastPercent = percent;
        };
        await conversion.execute();
        if (!target.buffer?.byteLength) throw new Error("MP4 转码未生成有效视频。");
        self.postMessage({ type: "complete", blob: new Blob([target.buffer], { type: "video/mp4" }) });
    } catch (error) {
        self.postMessage({ type: "error", message: error instanceof Error ? error.message : "MP4 转码失败" });
    } finally {
        if (output.state !== "finalized") await output.cancel().catch(() => {});
        input.dispose();
    }
};
