import { type CSSProperties, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import i18n from "@/i18n";
import { ImageSettingsTheme } from "@/components/image-settings-panel";
import { boolConfig, isSeedanceVideoConfig, normalizeSeedanceDuration, normalizeSeedanceRatio, normalizeSeedanceResolution, seedanceResolutionOptions } from "@/lib/seedance-video";
import { type CanvasTheme } from "@/lib/canvas-theme";
import { type AiConfig } from "@/stores/use-config-store";

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
export const videoSecondOptions = secondOptions.map((value) => String(value));

type VideoSettingsPanelProps = {
    config: AiConfig;
    onConfigChange: (key: "vquality" | "size" | "videoSeconds" | "videoAspectRatio" | "videoGenerateAudio" | "videoWatermark", value: string) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    className?: string;
};

export function VideoSettingsPanel({ config, onConfigChange, theme, showTitle = true, className = "w-[320px] space-y-4 rounded-2xl px-1 py-0.5" }: VideoSettingsPanelProps) {
    const { t } = useTranslation();
    const supportsGenerateAudio = isSeedanceVideoConfig(config);
    const rawRatio = supportsGenerateAudio ? normalizeSeedanceRatio(config.size) : normalizeVideoAspectRatio(config.videoAspectRatio);
    const ratio = canvasRatioOptions.includes(rawRatio as (typeof canvasRatioOptions)[number]) ? rawRatio : "16:9";
    const resolution = supportsGenerateAudio ? normalizeSeedanceResolution(config.vquality) : normalizeVideoResolutionValue(config.vquality);
    const resolutionOptionsForModel = supportsGenerateAudio ? seedanceResolutionOptions : resolutionOptions;
    const durationRange = supportsGenerateAudio ? { min: 4, max: 15 } : { min: 1, max: 20 };
    const configuredDuration = supportsGenerateAudio ? normalizeSeedanceDuration(config.videoSeconds) : Math.floor(Number(config.videoSeconds) || 6);
    const duration = Math.max(durationRange.min, Math.min(durationRange.max, configuredDuration));
    const sliderProgress = ((duration - durationRange.min) / (durationRange.max - durationRange.min)) * 100;
    const selectRatio = (value: (typeof canvasRatioOptions)[number]) => {
        onConfigChange("videoAspectRatio", value);
        if (supportsGenerateAudio) onConfigChange("size", value);
    };

    return (
        <ImageSettingsTheme theme={theme}>
            <div className={className} style={{ color: theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
                {showTitle ? <div className="text-lg font-semibold">{t("settingsPanels.video.title")}</div> : null}
                <SettingGroup title={t("settingsPanels.video.ratio")} color={theme.node.muted}>
                    <div className="rounded-xl p-1" style={{ background: theme.node.fill }}>
                        <div className="grid grid-cols-6 gap-1">
                            {canvasRatioOptions.map((item) => (
                                <CanvasRatioOption key={item} value={item} selected={ratio === item} theme={theme} onClick={() => selectRatio(item)} />
                            ))}
                        </div>
                    </div>
                </SettingGroup>
                <SettingGroup title={t("settingsPanels.video.resolution")} color={theme.node.muted}>
                    <SegmentedOptions
                        options={resolutionOptionsForModel}
                        value={resolution}
                        theme={theme}
                        onChange={(value) => onConfigChange("vquality", value)}
                    />
                </SettingGroup>
                <SettingGroup title={t("settingsPanels.video.duration")} color={theme.node.muted}>
                    <div className="flex items-center gap-4">
                        <input
                            type="range"
                            min={durationRange.min}
                            max={durationRange.max}
                            step={1}
                            value={duration}
                            aria-label={t("settingsPanels.video.duration")}
                            className="canvas-video-duration-range h-5 min-w-0 flex-1 cursor-pointer appearance-none bg-transparent"
                            style={{ "--video-slider-progress": `${sliderProgress}%`, "--video-slider-track": theme.node.stroke, "--video-slider-accent": theme.canvas.background === "#181715" ? "#22d3ee" : "#0891b2" } as CSSProperties}
                            onMouseDown={(event) => event.stopPropagation()}
                            onChange={(event) => onConfigChange("videoSeconds", event.target.value)}
                        />
                        <span className="grid h-9 min-w-12 place-items-center rounded-lg px-2 text-sm font-semibold" style={{ background: theme.node.fill, color: theme.node.text }}>
                            {duration}s
                        </span>
                    </div>
                </SettingGroup>
                {supportsGenerateAudio ? (
                    <SettingGroup title={t("settingsPanels.video.generateAudio")} color={theme.node.muted}>
                        <SegmentedOptions
                            options={[
                                { value: "true", label: t("settingsPanels.video.enabled") },
                                { value: "false", label: t("settingsPanels.video.disabled") },
                            ]}
                            value={String(boolConfig(config.videoGenerateAudio, true))}
                            theme={theme}
                            onChange={(value) => onConfigChange("videoGenerateAudio", value)}
                        />
                    </SettingGroup>
                ) : null}
            </div>
        </ImageSettingsTheme>
    );
}

export function videoResolutionLabel(value: string) {
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
    if (String(value).trim() === "-1") return i18n.t("settingsPanels.video.smart");
    return `${value || "6"}s`;
}

export function videoAspectRatioLabel(value: string) {
    return normalizeVideoAspectRatio(value);
}

export function normalizeVideoAspectRatio(value: string) {
    return canvasRatioOptions.includes(value as (typeof canvasRatioOptions)[number]) ? value : "16:9";
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

function CanvasRatioOption({ value, selected, theme, onClick }: { value: (typeof canvasRatioOptions)[number]; selected: boolean; theme: CanvasTheme; onClick: () => void }) {
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
            <span>{value}</span>
        </button>
    );
}

function SegmentedOptions({ options, value, theme, onChange }: { options: readonly { value: string; label: string }[]; value: string; theme: CanvasTheme; onChange: (value: string) => void }) {
    return (
        <div className="grid w-full rounded-xl p-1" style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))`, background: theme.node.fill }}>
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
