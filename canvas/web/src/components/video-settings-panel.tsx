import { useEffect, type CSSProperties, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import i18n from "@/i18n";
import { isXiaowenVideoConfig, normalizeXiaowenResolution } from "@/lib/xiaowen-video";
import { ImageSettingsTheme } from "@/components/image-settings-panel";
import { boolConfig, normalizeSeedanceRatio } from "@/lib/seedance-video";
import { type CanvasTheme } from "@/lib/canvas-theme";
import { type AiConfig } from "@/stores/use-config-store";
import { normalizeVideoConfig, videoDurations, videoProfile, videoProfiles } from "@/lib/video-models";

const resolutionOptions = [
    { value: "720", label: "720p" },
    { value: "480", label: "480p" },
];

const sizeOptions = [
    { value: "1280x720", labelKey: "landscape", width: 1280, height: 720 },
    { value: "720x1280", labelKey: "portrait", width: 720, height: 1280 },
    { value: "1024x1024", labelKey: "square", width: 1024, height: 1024 },
    { value: "1792x1024", labelKey: "widescreen", width: 1792, height: 1024 },
    { value: "1024x1792", labelKey: "tall", width: 1024, height: 1792 },
    { value: "auto", labelKey: "auto", width: 0, height: 0 },
];

const secondOptions = [6, 10, 12, 16, 20];
const canvasRatioOptions = ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16"] as const;
const seedanceRatioLabelKeys: Record<string, string> = { "16:9": "landscape", "9:16": "portrait", "1:1": "square", "4:3": "standardLandscape", "3:4": "standardPortrait", "21:9": "cinematic", adaptive: "adaptive" };

export const videoResolutionOptions = resolutionOptions.map((item) => ({ value: item.value, label: item.label }));
export const videoSizeOptions = sizeOptions.map((item) => ({ value: item.value, get label() { return i18n.t(`settingsPanels.video.sizes.${item.labelKey}`); } }));
export const videoSecondsRange = { min: 1, max: 20 };
export function supportsVideoGenerationMode(config: AiConfig) { return videoProfile(config).modes.length > 1; }
export function videoModeLabel(value: string) { return ({ auto: "自动", frames: "首尾帧", reference: "全能参考", edit: "视频编辑", extend: "视频延长" } as Record<string, string>)[value] || "自动"; }

export const videoSecondOptions = secondOptions.map((value) => String(value));

type VideoSettingsPanelProps = {
    config: AiConfig;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    className?: string;
};

export function VideoSettingsPanel({ config, onConfigChange, theme, showTitle = true, className = "w-[320px] space-y-4 rounded-2xl px-1 py-0.5" }: VideoSettingsPanelProps) {
    const { t } = useTranslation();
    const profile = videoProfile(config);
    const normalized = normalizeVideoConfig(config);
    const durations = videoDurations(normalized);
    const duration = Number(normalized.videoSeconds);
    const sliderIndex = Math.max(0, durations.indexOf(duration));
    const sliderProgress = durations.length > 1 ? sliderIndex / (durations.length - 1) * 100 : 0;
    const controlStyle = { background: theme.toolbar.panel, color: theme.node.text, borderColor: theme.node.stroke };
    useEffect(() => {
        for (const key of ["vquality", "videoSeconds", "videoAspectRatio", "videoMode"] as const) {
            if (normalized[key] !== config[key]) onConfigChange(key, normalized[key]);
        }
    }, [config.model, config.videoStandard, config.vquality, config.videoMode, config.videoSeconds, config.videoAspectRatio]);

    return (
        <ImageSettingsTheme theme={theme}>
            <div className={className} style={{ color: theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
                {showTitle ? <div className="text-lg font-semibold">{t("settingsPanels.video.title")}</div> : null}
                <SettingGroup title="接口标准" color={theme.node.muted}>
                    <select aria-label="接口标准" className="h-9 w-full rounded-md border px-2 text-sm" style={controlStyle} value={config.videoStandard || "auto"} onChange={(e) => onConfigChange("videoStandard", e.target.value)}>
                        <option value="auto">自动识别 · {videoProfile({ ...config, videoStandard: "auto" }).label}</option>
                        {videoProfiles.map((p) => <option key={p.id} value={p.id}>{p.label}</option>)}
                    </select>
                </SettingGroup>
                <SettingGroup title="请求格式" color={theme.node.muted}>
                    <SegmentedOptions options={[{ value: "auto", label: "自动" }, { value: "relay", label: "兼容网关" }, { value: "native", label: "厂家原生" }]} value={config.videoTransport || "auto"} theme={theme} onChange={(v) => onConfigChange("videoTransport", v)} />
                </SettingGroup>
                {supportsVideoGenerationMode(config) && <SettingGroup title={t("settingsPanels.video.mode")} color={theme.node.muted}>
                    <SegmentedOptions options={profile.modes.map((value) => ({ value, label: videoModeLabel(value) }))} value={normalized.videoMode} theme={theme} onChange={(value) => onConfigChange("videoMode", value)} />
                </SettingGroup>}
                {profile.ratios.length > 0 && <SettingGroup title={t("settingsPanels.video.ratio")} color={theme.node.muted}>
                    <div className="rounded-xl p-1" style={{ background: theme.node.fill }}>
                        <div className="grid grid-cols-4 gap-1">
                            {profile.ratios.map((item) => (
                                <CanvasRatioOption key={item} value={item} selected={normalized.videoAspectRatio === item} theme={theme} onClick={() => onConfigChange("videoAspectRatio", item)} />
                            ))}
                        </div>
                    </div>
                </SettingGroup>}
                {profile.resolutions.length > 0 && <SettingGroup title={t("settingsPanels.video.resolution")} color={theme.node.muted}>
                    <SegmentedOptions
                        options={profile.resolutions.map((value) => ({ value, label: /k/i.test(value) ? value.toUpperCase() : `${value}p` }))}
                        value={normalized.vquality}
                        theme={theme}
                        onChange={(value) => onConfigChange("vquality", value)}
                    />
                </SettingGroup>}
                {durations.length > 0 && <SettingGroup title={t("settingsPanels.video.duration")} color={theme.node.muted}>
                    <div className="flex items-center gap-4">
                        <input
                            type="range"
                            min={0}
                            max={Math.max(0, durations.length - 1)}
                            step={1}
                            value={sliderIndex}
                            disabled={durations.length === 1}
                            aria-label={t("settingsPanels.video.duration")}
                            className="canvas-video-duration-range h-5 min-w-0 flex-1 cursor-pointer appearance-none bg-transparent"
                            style={{ "--video-slider-progress": `${sliderProgress}%`, "--video-slider-track": theme.node.stroke, "--video-slider-accent": theme.canvas.background === "#181715" ? "#22d3ee" : "#0891b2" } as CSSProperties}
                            onMouseDown={(event) => event.stopPropagation()}
                            onChange={(event) => onConfigChange("videoSeconds", String(durations[Number(event.target.value)]))}
                        />
                        <span className="grid h-9 min-w-12 place-items-center rounded-lg px-2 text-sm font-semibold" style={{ background: theme.node.fill, color: theme.node.text }}>
                            {duration === -1 ? "随源视频" : `${duration}s`}
                        </span>
                    </div>
                </SettingGroup>}
                {profile.protocol === "kling" && <SettingGroup title="生成档位" color={theme.node.muted}><SegmentedOptions options={[{ value: "std", label: "标准" }, { value: "pro", label: "专业" }]} value={config.videoKlingMode || "pro"} theme={theme} onChange={(v) => onConfigChange("videoKlingMode", v)} /></SettingGroup>}
                {profile.audio ? (
                    <SettingGroup title={profile.id === "kling-o1" ? "保留源音频" : t("settingsPanels.video.generateAudio")} color={theme.node.muted}>
                        <input type="checkbox" aria-label={t("settingsPanels.video.generateAudio")} checked={boolConfig(config.videoGenerateAudio, true)} onChange={(e) => onConfigChange("videoGenerateAudio", String(e.target.checked))} />
                    </SettingGroup>
                ) : null}
                {profile.watermark && <SettingGroup title="水印" color={theme.node.muted}><input type="checkbox" aria-label="水印" checked={boolConfig(config.videoWatermark, false)} onChange={(e) => onConfigChange("videoWatermark", String(e.target.checked))} /></SettingGroup>}
                {profile.negative && <SettingGroup title="负面提示词" color={theme.node.muted}><textarea aria-label="负面提示词" className="w-full resize-y rounded-md border p-2 text-sm" style={controlStyle} rows={2} value={config.videoNegativePrompt || ""} onChange={(e) => onConfigChange("videoNegativePrompt", e.target.value)} /></SettingGroup>}
                {["ark", "veo", "relay"].includes(profile.protocol) && <SettingGroup title="随机种子" color={theme.node.muted}><input type="number" aria-label="随机种子" placeholder="随机" className="h-9 w-full rounded-md border px-2 text-sm" style={controlStyle} value={config.videoSeed || ""} onChange={(e) => onConfigChange("videoSeed", e.target.value)} /></SettingGroup>}
                <div className="flex items-center justify-between text-xs" style={{ color: theme.node.muted }}>
                    <span>参考上限 图 {Number.isFinite(profile.images) ? profile.images : "未限定"} / 视频 {Number.isFinite(profile.videos) ? profile.videos : "未限定"} / 音频 {Number.isFinite(profile.audios) ? profile.audios : "未限定"}</span>
                    <label className="inline-flex items-center gap-1"><input type="checkbox" checked={config.videoValidation === "strict"} onChange={(e) => onConfigChange("videoValidation", e.target.checked ? "strict" : "warn")} />严格校验</label>
                </div>
            </div>
        </ImageSettingsTheme>
    );
}

export function videoResolutionLabel(value: string, config?: AiConfig) {
    if (config && isXiaowenVideoConfig(config)) return normalizeXiaowenResolution(value);
    if (/^[24]k$/i.test(value.trim())) return value.toUpperCase();
    return `${normalizeVideoResolutionValue(value)}p`;
}

export function videoSizeLabel(value: string) {
    const ratio = normalizeSeedanceRatio(value);
    if (value === "adaptive" || value === "auto") return i18n.t("settingsPanels.video.adaptive");
    if (ratio === value) return i18n.t(`settingsPanels.video.ratios.${seedanceRatioLabelKeys[ratio]}`);
    const size = normalizeVideoSizeValue(value);
    const option = sizeOptions.find((item) => item.value === size);
    return option ? i18n.t(`settingsPanels.video.sizes.${option.labelKey}`) : size;
}

export function videoSecondsLabel(value: string) {
    if (String(value).trim() === "-1") return "随源视频";
    return `${value || "6"}s`;
}

export function videoAspectRatioLabel(value: string) {
    return normalizeVideoAspectRatio(value);
}

export function normalizeVideoAspectRatio(value: string) {
    return value === "adaptive" ? "自适应" : canvasRatioOptions.includes(value as (typeof canvasRatioOptions)[number]) ? value : "16:9";
}

export function normalizeVideoSizeValue(value: string) {
    if (value === "auto") return "auto";
    if (/^\d+x\d+$/.test(value || "")) return value;
    return ["9:16", "2:3", "3:4"].includes(value) ? "720x1280" : "1280x720";
}

export function normalizeVideoResolutionValue(value: string) {
    if (value === "480p" || value === "low") return "480";
    if (value === "720p" || value === "auto" || value === "high" || value === "medium") return "720";
    return value.replace(/p$/i, "") || "720";
}

function CanvasRatioOption({ value, selected, theme, onClick }: { value: string; selected: boolean; theme: CanvasTheme; onClick: () => void }) {
    const preview = ratioPreview(value);
    const longSide = Math.max(preview.width, preview.height);
    const previewWidth = Math.max(10, Math.round((preview.width / longSide) * 26));
    const previewHeight = Math.max(10, Math.round((preview.height / longSide) * 26));

    return (
        <button
            type="button"
            aria-pressed={selected}
            className="flex h-[84px] min-w-0 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg text-sm transition hover:opacity-80"
            style={{ background: selected ? theme.toolbar.activeBg : "transparent", color: theme.node.text }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={onClick}
        >
            <span className="grid h-6 place-items-center">
                <span className="border-2" style={{ width: previewWidth, height: previewHeight, borderColor: theme.node.text }} />
            </span>
            <span>{value === "adaptive" ? "自适应" : value}</span>
        </button>
    );
}

function SegmentedOptions({ options, value, theme, onChange }: { options: readonly { value: string; label: string }[]; value: string; theme: CanvasTheme; onChange: (value: string) => void }) {
    return (
        <div className="grid w-full gap-1 rounded-lg p-1" style={{ gridTemplateColumns: `repeat(${Math.min(options.length, 4)}, minmax(0, 1fr))`, background: theme.node.fill }}>
            {options.map((option) => {
                const selected = option.value === value;
                return (
                    <button
                        key={option.value}
                        type="button"
                        aria-pressed={selected}
                        className="h-9 min-w-0 rounded-lg px-2 text-sm font-medium transition hover:opacity-80"
                        style={{ background: selected ? theme.toolbar.activeBg : "transparent", color: selected ? theme.node.text : theme.node.muted }}
                        onMouseDown={(event) => event.stopPropagation()}
                        onClick={() => onChange(option.value)}
                    >
                        {option.label}
                    </button>
                );
            })}
        </div>
    );
}

function SettingGroup({ title, color, children }: { title: string; color: string; children: ReactNode }) {
    return (
        <div className="space-y-2.5">
            <div className="text-xs font-medium" style={{ color }}>
                {title}
            </div>
            {children}
        </div>
    );
}

function ratioPreview(ratio: string) {
    if (ratio === "9:16") return { width: 9, height: 16 };
    if (ratio === "1:1") return { width: 1, height: 1 };
    if (ratio === "4:3") return { width: 4, height: 3 };
    if (ratio === "3:4") return { width: 3, height: 4 };
    if (ratio === "21:9") return { width: 21, height: 9 };
    return { width: 16, height: 9 };
}
