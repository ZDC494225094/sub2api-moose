import { modelOptionName, resolveModelRequestConfig, type AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";

export type VideoProtocol = "relay" | "ark" | "minimax" | "minimax-h3" | "veo" | "omni" | "kling" | "kling-v3";
export type VideoProfile = {
    id: string; label: string; match: RegExp; protocol: VideoProtocol;
    resolutions: string[]; ratios: string[]; durations: number[];
    modes: string[]; images: number; videos: number; audios: number;
    total?: number; videoSeconds?: number; audioSeconds?: number;
    audio?: boolean; watermark?: boolean; negative?: boolean;
};
const range = (min: number, max: number) => Array.from({ length: max - min + 1 }, (_, i) => i + min);
const ratios = ["16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"];
const frames = ["auto", "frames"];
const multimodal = ["auto", "frames", "reference"];
// Keep generations separate: a vendor name is not a capability guarantee.
export const videoProfiles: VideoProfile[] = [
    { id: "seedance-2.5", label: "Seedance 2.5", match: /seedance[-_. ]2[-_. ]5/i, protocol: "ark", resolutions: ["480", "720", "1080"], ratios, durations: range(4, 30), modes: [...multimodal, "edit", "extend"], images: 30, videos: 10, audios: 10, videoSeconds: 30, audioSeconds: 30, audio: true, watermark: true },
    { id: "seedance-2-fast", label: "Seedance 2.0 Fast / Mini", match: /seedance[-_. ]2(?:[-_. ]0)?[-_. ](?:fast|mini)/i, protocol: "ark", resolutions: ["480", "720"], ratios, durations: range(4, 15), modes: multimodal, images: 9, videos: 3, audios: 3, total: 12, videoSeconds: 15, audioSeconds: 15, audio: true, watermark: true },
    { id: "seedance-2", label: "Seedance 2.0", match: /seedance[-_. ]2(?:[-_. ]0)?(?:[-_. ]|$)/i, protocol: "ark", resolutions: ["480", "720", "1080", "4k"], ratios, durations: range(4, 15), modes: multimodal, images: 9, videos: 3, audios: 3, total: 12, videoSeconds: 15, audioSeconds: 15, audio: true, watermark: true },
    { id: "seedance-1.5", label: "Seedance 1.5 Pro", match: /seedance[-_. ]1[-_. ]5/i, protocol: "ark", resolutions: ["480", "720"], ratios, durations: range(4, 12), modes: frames, images: 2, videos: 0, audios: 0, audio: true, watermark: true },
    { id: "seedance-1-fast", label: "Seedance 1.0 Pro Fast", match: /seedance[-_. ]1[-_. ]0[-_. ]pro[-_. ]fast/i, protocol: "ark", resolutions: ["480", "720", "1080"], ratios, durations: range(2, 12), modes: frames, images: 1, videos: 0, audios: 0, watermark: true },
    { id: "seedance-1", label: "Seedance 1.0 Pro", match: /seedance[-_. ]1[-_. ]0[-_. ]pro/i, protocol: "ark", resolutions: ["480", "720", "1080"], ratios, durations: range(2, 12), modes: frames, images: 2, videos: 0, audios: 0, watermark: true },
    { id: "minimax-h3-max", label: "MiniMax H3 Max", match: /minimax[-_. ]h3[-_. ]max/i, protocol: "minimax-h3", resolutions: ["480", "768"], ratios, durations: range(5, 15), modes: multimodal, images: 9, videos: 3, audios: 3, total: 12, videoSeconds: 15, audioSeconds: 15 },
    { id: "minimax-h3", label: "MiniMax H3", match: /minimax[-_. ]h3(?:$|[-_. ])|h3video|xiaowen/i, protocol: "minimax-h3", resolutions: ["768", "2K"], ratios, durations: range(4, 15), modes: multimodal, images: 9, videos: 3, audios: 3, total: 12, videoSeconds: 15, audioSeconds: 15 },
    { id: "hailuo-2.3-fast", label: "Hailuo 2.3 Fast（图生）", match: /hailuo[-_. ]2[-_. ]3[-_. ]fast/i, protocol: "minimax", resolutions: ["768", "1080"], ratios: [], durations: [6, 10], modes: frames, images: 1, videos: 0, audios: 0 },
    { id: "hailuo-2.3", label: "Hailuo 2.3", match: /hailuo[-_. ]2[-_. ]3/i, protocol: "minimax", resolutions: ["768", "1080"], ratios: [], durations: [6, 10], modes: frames, images: 1, videos: 0, audios: 0 },
    { id: "hailuo-02", label: "Hailuo 02（首尾帧）", match: /hailuo[-_. ]02/i, protocol: "minimax", resolutions: ["512", "768", "1080"], ratios: [], durations: [6, 10], modes: frames, images: 2, videos: 0, audios: 0 },
    { id: "minimax-01", label: "MiniMax T2V / I2V 01", match: /(?:^|\/)(?:t2v|i2v)-01/i, protocol: "minimax", resolutions: ["720"], ratios: [], durations: [6], modes: frames, images: 1, videos: 0, audios: 0 },
    { id: "veo-3.1-lite", label: "Google Veo 3.1 Lite", match: /veo[-_. ]3[-_. ]1[-_. ]lite/i, protocol: "veo", resolutions: ["720", "1080"], ratios: ["16:9", "9:16"], durations: [4, 6, 8], modes: frames, images: 2, videos: 0, audios: 0, negative: true },
    { id: "veo-3.1", label: "Google Veo 3.1 / Fast", match: /veo[-_. ]3[-_. ]1/i, protocol: "veo", resolutions: ["720", "1080", "4k"], ratios: ["16:9", "9:16"], durations: [4, 6, 8], modes: [...multimodal, "extend"], images: 3, videos: 1, audios: 0, negative: true },
    { id: "veo-3", label: "Google Veo 3 / Fast", match: /veo[-_. ]3(?:[-_. ]|$)/i, protocol: "veo", resolutions: ["720", "1080"], ratios: ["16:9", "9:16"], durations: [8], modes: frames, images: 1, videos: 0, audios: 0, negative: true },
    { id: "veo-2", label: "Google Veo 2", match: /veo[-_. ]2(?:[-_. ]|$)/i, protocol: "veo", resolutions: ["720"], ratios: ["16:9", "9:16"], durations: [5, 6, 7, 8], modes: frames, images: 1, videos: 0, audios: 0, negative: true },
    { id: "gemini-omni", label: "Gemini Omni Flash", match: /gemini[-_. ]omni/i, protocol: "omni", resolutions: ["360", "720", "1080", "4k"], ratios: ["16:9", "9:16"], durations: [], modes: [...multimodal, "edit", "extend"], images: 6, videos: 3, audios: 0 },
    { id: "kling-o1", label: "可灵 O1（新版）", match: /kling[-_. ](?:v[-_. ]?)?o1/i, protocol: "kling-v3", resolutions: ["720", "1080"], ratios: ["16:9", "9:16", "1:1"], durations: range(3, 10), modes: [...multimodal, "edit"], images: 7, videos: 1, audios: 0, audio: true, watermark: true },
    { id: "kling-3-turbo", label: "可灵 3.0 Turbo", match: /kling.*(?:v?3[-_. ]0|v3).*turbo/i, protocol: "kling-v3", resolutions: ["720", "1080"], ratios: ["16:9", "9:16", "1:1"], durations: range(3, 15), modes: frames, images: 1, videos: 0, audios: 0, watermark: true },
    { id: "kling-3-omni", label: "可灵 3.0 Omni（新版）", match: /kling.*(?:v?3[-_. ]0|v3).*omni|kling.*omni.*3/i, protocol: "kling-v3", resolutions: ["720", "1080", "4k"], ratios: ["16:9", "9:16", "1:1"], durations: range(3, 15), modes: [...multimodal, "edit"], images: 7, videos: 1, audios: 0, audio: true, watermark: true },
    { id: "kling-3", label: "可灵 3.0（新版）", match: /^(?!.*turbo).*kling.*(?:v?3[-_. ]0|v3)(?:[-_. ]|$)/i, protocol: "kling-v3", resolutions: ["720", "1080", "4k"], ratios: ["16:9", "9:16", "1:1"], durations: range(3, 15), modes: frames, images: 2, videos: 0, audios: 0, audio: true, watermark: true },
    { id: "kling-2.6", label: "可灵 2.6", match: /kling.*(?:v?2[-_. ]6)/i, protocol: "kling", resolutions: [], ratios: ["16:9", "9:16", "1:1"], durations: [5, 10], modes: frames, images: 1, videos: 0, audios: 0, audio: true, negative: true },
    { id: "kling-2.5", label: "可灵 2.5 Turbo", match: /kling.*v?2[-_. ]5/i, protocol: "kling", resolutions: [], ratios: ["16:9", "9:16", "1:1"], durations: [5, 10], modes: frames, images: 2, videos: 0, audios: 0, negative: true },
    { id: "kling-legacy", label: "可灵 1.x / 2.1", match: /kling.*v?(?:1[-_. ](?:0|5|6)|2[-_. ]1)/i, protocol: "kling", resolutions: [], ratios: ["16:9", "9:16", "1:1"], durations: [5, 10], modes: frames, images: 2, videos: 0, audios: 0, negative: true },
    { id: "sora-2", label: "Sora 2 / Pro（历史兼容）", match: /sora-2/i, protocol: "relay", resolutions: ["720", "1024"], ratios: ["16:9", "9:16"], durations: [4, 8, 12], modes: frames, images: 1, videos: 0, audios: 0 },
    { id: "custom", label: "通用 / 自定义", match: /$^/, protocol: "relay", resolutions: ["360", "480", "512", "720", "768", "1080", "2K", "4k"], ratios, durations: range(1, 60), modes: [...multimodal, "edit", "extend"], images: Infinity, videos: Infinity, audios: Infinity, audio: true, watermark: true, negative: true },
];
export function videoProfile(config: AiConfig) {
    const model = modelOptionName(config.model || config.videoModel).split("/").pop() || "";
    return videoProfiles.find((p) => p.id === config.videoStandard) || videoProfiles.find((p) => p.match.test(model)) || videoProfiles[videoProfiles.length - 1];
}
export function videoMode(config: AiConfig, images: number, videos = 0, audios = 0) {
    if (config.videoMode && config.videoMode !== "auto") return config.videoMode;
    if (videoProfile(config).protocol === "veo" && videos) return "extend";
    return images > 2 || videos > 0 || audios > 0 ? "reference" : "frames";
}
export function videoDurations(config: AiConfig) {
    const p = videoProfile(config);
    if (p.id === "seedance-2.5" && config.videoMode === "edit") return [-1];
    if (/^veo-3/.test(p.id) && (config.videoMode === "reference" || config.videoMode === "extend" || /1080|4k/i.test(config.vquality))) return [8];
    if (p.protocol === "minimax" && /1080/.test(config.vquality)) return [6];
    return p.durations;
}
export function normalizeVideoConfig(config: AiConfig): AiConfig {
    const p = videoProfile(config);
    const resolution = config.vquality.replace(/p$/i, "");
    const next = { ...config, vquality: p.resolutions.includes(resolution) ? resolution : p.resolutions[0] || resolution, videoAspectRatio: p.ratios.includes(config.videoAspectRatio) ? config.videoAspectRatio : p.ratios[0] || "16:9", videoMode: p.modes.includes(config.videoMode) ? config.videoMode : "auto" };
    if (p.id === "seedance-2.5" && ["edit", "extend"].includes(next.videoMode)) next.videoAspectRatio = "adaptive";
    const durations = videoDurations(next);
    next.videoSeconds = durations.includes(Number(next.videoSeconds)) ? next.videoSeconds : String(durations[0] || 8);
    return next;
}
export function videoRequestProtocol(config: AiConfig): VideoProtocol {
    if (config.videoTransport === "relay") return "relay";
    const request = resolveModelRequestConfig(config, config.model || config.videoModel);
    if (config.videoTransport === "native") return videoProfile(config).protocol;
    let url: URL;
    try { url = new URL(request.baseUrl); } catch { return "relay"; }
    const host = url.hostname.toLowerCase();
    if (host.endsWith(".volces.com") || /\/api\/v3\/?$/.test(url.pathname)) return "ark";
    if (["api.minimax.io", "api.minimax.chat", "api.minimaxi.com"].includes(host)) return videoProfile(config).protocol === "minimax-h3" ? "minimax-h3" : "minimax";
    if (["api.klingai.com", "api-singapore.klingai.com", "api-beijing.klingai.com"].includes(host)) return videoProfile(config).protocol;
    if (request.apiFormat === "gemini" || host === "generativelanguage.googleapis.com") return videoProfile(config).protocol === "omni" ? "omni" : "veo";
    return "relay";
}
export function prepareVideoConfig(config: AiConfig, images: number, videos = 0, audios = 0) {
    const next = normalizeVideoConfig(config);
    if (next.videoSeed.trim() && (!Number.isSafeInteger(Number(next.videoSeed)) || Number(next.videoSeed) < -1)) throw new Error("随机种子需为 -1 或非负安全整数。");
    const mode = videoMode(next, images, videos, audios), profile = videoProfile(next);
    const prepared = normalizeVideoConfig({ ...next, videoMode: mode, ...(profile.protocol === "veo" && videos ? { vquality: "720" } : {}) });
    if (profile.id === "seedance-2.5" && mode === "frames" && images) prepared.videoAspectRatio = "adaptive";
    if (profile.id === "kling-o1" && mode === "frames" && images && !videos && ![5, 10].includes(Number(prepared.videoSeconds))) prepared.videoSeconds = "5";
    return { ...prepared, videoMode: mode };
}
export function videoReferenceCountWarning(config: AiConfig, images: number, videos: number, audios: number) {
    const p = videoProfile(config), mode = videoMode(config, images, videos, audios);
    const maxImages = mode === "frames" ? Math.min(2, p.images) : ["kling-3-omni", "kling-o1"].includes(p.id) && videos ? 4 : p.images;
    if (images > maxImages || videos > p.videos || audios > p.audios || (p.total && images + videos + audios > p.total)) return `${p.label}：参考上限 图 ${maxImages} / 视频 ${p.videos} / 音频 ${p.audios}${p.total ? `，合计 ${p.total}` : ""}；当前 ${images} / ${videos} / ${audios}`;
    if (mode === "frames" && (videos || audios)) return "首尾帧不能混入音视频，请切换全能参考。";
    if (["edit", "extend"].includes(mode) && videos !== 1) return "编辑 / 延长需要一个源视频。";
    if (p.protocol === "veo" && videos && (images || mode !== "extend")) return "Veo 视频仅用于延长，不能混用参考图。";
    if (p.id === "hailuo-2.3-fast" && !images) return "Hailuo 2.3 Fast 需要一张首帧图。";
    if (p.id.startsWith("seedance-2") && p.id !== "seedance-2.5" && audios && !images && !videos) return "Seedance 2.0 音频参考需搭配图片或视频。";
    if (p.id === "hailuo-02" && !images && config.vquality === "512") return "Hailuo 02 的 512p 仅适用于图生视频。";
    return "";
}
export function videoReferenceWarning(config: AiConfig, images: ReferenceImage[], videos: ReferenceVideo[], audios: ReferenceAudio[]) {
    const p = videoProfile(config);
    const counts = videoReferenceCountWarning(config, images.length, videos.length, audios.length);
    if (counts) return counts;
    if (p.protocol === "ark" && videos.some((item) => item.type === "video/webm" || /\.webm$/i.test(item.name))) return "Seedance 官方视频参考支持 MP4 / MOV，WebM 深度视频需转码或使用支持 WebM 的网关。";
    if (p.protocol === "kling-v3" && videos.some((item) => item.durationMs && (item.durationMs < 3000 || item.durationMs > (p.id === "kling-o1" ? 10000 : 15500)))) return `${p.label} 参考视频需为 3–${p.id === "kling-o1" ? "10" : "15.5"} 秒。`;
    if (p.id === "seedance-2.5" && videos.some((item) => item.durationMs && item.durationMs < (config.videoMode === "edit" ? 4000 : 2000))) return "Seedance 2.5 参考视频至少 2 秒，编辑源视频至少 4 秒。";
    if (p.id === "gemini-omni" && videos.some((item) => (item.durationMs || 0) > (config.videoMode === "reference" ? 3000 : 10000))) return "Gemini Omni：参考视频每段最多 3 秒，编辑 / 延长源视频最多 10 秒。";
    if (p.protocol === "minimax-h3" && [...videos, ...audios].some((item) => item.durationMs && (item.durationMs < 2000 || item.durationMs > 15000))) return "MiniMax H3 的单段音视频参考需为 2–15 秒。";
    if (p.protocol === "minimax-h3" && videos.some((item) => (item.bytes || 0) > 50 * 1024 * 1024)) return "MiniMax H3 单个参考视频不能超过 50 MB。";
    for (const [items, limit, label] of [[videos, p.videoSeconds, "视频"], [audios, p.audioSeconds, "音频"]] as const) {
        if (limit && items.reduce((sum, item) => sum + (item.durationMs || 0) / 1000, 0) > limit) return `${p.label} 参考${label}总时长不能超过 ${limit} 秒。`;
    }
    return "";
}
export function assertVideoReferences(config: AiConfig, images: ReferenceImage[], videos: ReferenceVideo[], audios: ReferenceAudio[]) {
    const warning = videoReferenceWarning(config, images, videos, audios);
    // Relay aliases can expose extended capabilities. Never drop their media.
    if (warning && config.videoValidation === "strict") throw new Error(`${warning} 可切换接口标准或关闭严格校验。`);
}
