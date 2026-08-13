import { useEffect, useId, useMemo, useState } from "react";
import { ChevronRight, Cpu } from "lucide-react";
import { useTranslation } from "react-i18next";

import i18n from "@/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { getBrandingSiteName, useBrandingStore } from "@/stores/use-branding-store";
import {
    decodeChannelModel,
    encodeChannelModel,
    isManagedCanvasChannel,
    isSeedanceVideoAlias,
    modelOptionLabel,
    modelOptionName,
    selectableModelsByCapability,
    type AiConfig,
    type ModelCapability,
} from "@/stores/use-config-store";

type ModelPickerProps = {
    config: AiConfig;
    value?: string;
    onChange: (model: string) => void;
    capability?: ModelCapability;
    className?: string;
    fullWidth?: boolean;
    placeholder?: string;
    onMissingConfig?: () => void;
};

type ModelGroup = {
    id: string;
    label: string;
    models: string[];
};

export function ModelPicker({ config, value, onChange, capability, className, fullWidth = false, placeholder, onMissingConfig }: ModelPickerProps) {
    const { t } = useTranslation();
    const pickerId = useId();
    const [open, setOpen] = useState(false);
    const [expandedGroupId, setExpandedGroupId] = useState<string | null>(null);
    const siteName = useBrandingStore((state) => state.siteName);
    const options = useMemo(() => Array.from(new Set([...(config.channelMode === "local" && !capability ? [value] : []), ...selectableModelsByCapability(config, capability)].filter((model): model is string => Boolean(model)))), [capability, config, value]);
    const modelGroups = useMemo(() => {
        const remaining = new Set(options);
        const groups: ModelGroup[] = [];

        for (const channel of config.channels) {
            const models = channel.models
                .filter((model) => !capability || model.capability === capability)
                .map((model) => encodeChannelModel(channel.id, model.name))
                .filter((model) => {
                    if (!remaining.has(model)) return false;
                    remaining.delete(model);
                    return true;
                });
            if (!models.length) continue;

            groups.push({
                id: channel.id,
                label: isManagedCanvasChannel(channel) ? `${siteName || getBrandingSiteName()} / ${channel.managedGroup?.name || channel.name}` : channel.name,
                models,
            });
        }

        if (remaining.size) {
            groups.push({ id: "__other_models__", label: "其他模型", models: Array.from(remaining) });
        }
        return groups;
    }, [capability, config.channels, options, siteName]);
    const current = value || "";
    const pickerPlaceholder = placeholder || t("settingsPanels.model.select");

    useEffect(() => {
        const closeOtherPicker = (event: Event) => {
            if ((event as CustomEvent<string>).detail !== pickerId) setOpen(false);
        };
        window.addEventListener("model-picker-open", closeOtherPicker);
        return () => window.removeEventListener("model-picker-open", closeOtherPicker);
    }, [pickerId]);

    return (
        <Select
            open={open}
            value={current}
            onOpenChange={(nextOpen) => {
                if (nextOpen && !options.length && config.channelMode === "local") onMissingConfig?.();
                if (nextOpen) {
                    setExpandedGroupId(null);
                    window.dispatchEvent(new CustomEvent("model-picker-open", { detail: pickerId }));
                }
                setOpen(nextOpen);
            }}
            onValueChange={onChange}
        >
            <SelectTrigger
                className={cn(
                    "canvas-composer-model-picker h-8 w-fit max-w-full gap-2 rounded-full border border-input bg-transparent px-3 text-sm font-normal shadow-sm transition-colors",
                    fullWidth ? "w-full min-w-0 justify-start" : "min-w-[9rem] justify-start",
                    "data-[state=open]:border-ring data-[state=open]:ring-2 data-[state=open]:ring-ring/20",
                    className,
                )}
                onMouseDown={(event) => event.stopPropagation()}
                onPointerDown={(event) => event.stopPropagation()}
                title={current ? modelOptionLabel(config, current) : pickerPlaceholder}
            >
                <ModelIcon config={config} model={current} />
                <span className="canvas-model-picker-text min-w-0 flex-1 truncate text-left">{current ? modelOptionLabel(config, current) : pickerPlaceholder}</span>
            </SelectTrigger>
            <SelectContent
                data-canvas-no-zoom
                className="z-[1200] w-80 max-w-[calc(100vw-24px)] rounded-xl border border-border/70 bg-popover p-1 shadow-xl"
                position="popper"
                align="start"
                side="bottom"
                sideOffset={6}
                onPointerDown={(event) => event.stopPropagation()}
                onMouseDown={(event) => event.stopPropagation()}
            >
                {modelGroups.length ? (
                    modelGroups.map((group) => {
                        const expanded = expandedGroupId === group.id;
                        return (
                            <div key={group.id} className="py-0.5">
                                <button
                                    type="button"
                                    className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none"
                                    aria-expanded={expanded}
                                    onPointerDown={(event) => {
                                        event.preventDefault();
                                        event.stopPropagation();
                                    }}
                                    onClick={(event) => {
                                        event.preventDefault();
                                        event.stopPropagation();
                                        setExpandedGroupId((currentGroupId) => (currentGroupId === group.id ? null : group.id));
                                    }}
                                >
                                    <ChevronRight className={cn("size-4 shrink-0 text-muted-foreground transition-transform duration-150", expanded && "rotate-90")} />
                                    <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                                    <span className="min-w-5 rounded bg-muted px-1.5 py-0.5 text-center text-xs tabular-nums text-muted-foreground">{group.models.length}</span>
                                </button>
                                {expanded
                                    ? group.models.map((model) => (
                                        <SelectItem key={model} value={model} textValue={modelOptionLabel(config, model)} className="pl-8">
                                            <ModelLabel config={config} model={model} />
                                        </SelectItem>
                                    ))
                                    : null}
                            </div>
                        );
                    })
                ) : (
                    <SelectItem value="__empty__" disabled>
                        {emptyModelLabel(config, capability)}
                    </SelectItem>
                )}
            </SelectContent>
        </Select>
    );
}

function emptyModelLabel(config: AiConfig, capability?: ModelCapability) {
    const label = capability ? i18n.t(`settingsPanels.model.capabilities.${capability}`) : "";
    if (capability && config.models.length) return i18n.t("settingsPanels.model.assign", { capability: label });
    return config.models.length ? i18n.t("settingsPanels.model.noMatch", { capability: label }) : i18n.t("settingsPanels.model.addFirst");
}

function ModelLabel({ config, model }: { config: AiConfig; model: string }) {
    return (
        <span className="flex min-w-0 items-center gap-2">
            <ModelIcon config={config} model={model} />
            <span className="truncate">{modelOptionName(model)}</span>
        </span>
    );
}

function ModelIcon({ config, model }: { config?: AiConfig; model: string }) {
    const icon = resolveModelIcon(modelOptionName(model), config, model);
    if (icon === "openai") return <OpenAIModelIcon />;
    return icon ? <img src={icon} alt="" className={cn("size-4 shrink-0", isMonochromeIcon(icon) && "dark:invert")} /> : <Cpu className="size-4 shrink-0 opacity-70" />;
}

function isMonochromeIcon(icon: string) {
    return icon.endsWith("/icons/grok.svg");
}

function OpenAIModelIcon() {
    return (
        <svg width="14px" height="14px" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" className="model-icon shrink-0 dark:invert" fill="currentColor" fillRule="evenodd">
            <path d="M21.55 10.004a5.416 5.416 0 00-.478-4.501c-1.217-2.09-3.662-3.166-6.05-2.66A5.59 5.59 0 0010.831 1C8.39.995 6.224 2.546 5.473 4.838A5.553 5.553 0 001.76 7.496a5.487 5.487 0 00.691 6.5 5.416 5.416 0 00.477 4.502c1.217 2.09 3.662 3.165 6.05 2.66A5.586 5.586 0 0013.168 23c2.443.006 4.61-1.546 5.361-3.84a5.553 5.553 0 003.715-2.66 5.488 5.488 0 00-.693-6.497v.001zm-8.381 11.558a4.199 4.199 0 01-2.675-.954c.034-.018.093-.05.132-.074l4.44-2.53a.71.71 0 00.364-.623v-6.176l1.877 1.069c.02.01.033.029.036.05v5.115c-.003 2.274-1.87 4.118-4.174 4.123zM4.192 17.78a4.059 4.059 0 01-.498-2.763c.032.02.09.055.131.078l4.44 2.53c.225.13.504.13.73 0l5.42-3.088v2.138a.068.068 0 01-.027.057L9.9 19.288c-1.999 1.136-4.552.46-5.707-1.51h-.001zM3.023 8.216A4.15 4.15 0 015.198 6.41l-.002.151v5.06a.711.711 0 00.364.624l5.42 3.087-1.876 1.07a.067.067 0 01-.063.005l-4.489-2.559c-1.995-1.14-2.679-3.658-1.53-5.63h.001zm15.417 3.54l-5.42-3.088L14.896 7.6a.067.067 0 01.063-.006l4.489 2.557c1.998 1.14 2.683 3.662 1.529 5.633a4.163 4.163 0 01-2.174 1.807V12.38a.71.71 0 00-.363-.623zm1.867-2.773a6.04 6.04 0 00-.132-.078l-4.44-2.53a.731.731 0 00-.729 0l-5.42 3.088V7.325a.068.068 0 01.027-.057L14.1 4.713c2-1.137 4.555-.46 5.707 1.513.487.833.664 1.809.499 2.757h.001zm-11.741 3.81l-1.877-1.068a.065.065 0 01-.036-.051V6.559c.001-2.277 1.873-4.122 4.181-4.12.976 0 1.92.338 2.671.954-.034.018-.092.05-.131.073l-4.44 2.53a.71.71 0 00-.365.623l-.003 6.173v.002zm1.02-2.168L12 9.25l2.414 1.375v2.75L12 14.75l-2.415-1.375v-2.75z" fill="#000000" />
        </svg>
    );
}

function resolveModelIcon(model: string, config?: AiConfig, value?: string) {
    const channelId = value ? decodeChannelModel(value)?.channelId : undefined;
    const channel = config?.channels.find((item) => item.id === channelId);
    const modelEntry = channel?.models.find((entry) => entry.name === model);
    const platform = (modelEntry?.providerPlatform || channel?.providerPlatform || channel?.managedGroup?.platform || "").toLowerCase();
    const name = model.toLowerCase();
    if (name.includes("claude") || name.includes("anthropic")) return providerIcon("claude");
    if (name.includes("gemini") || name.includes("gemma") || name.includes("learnlm") || name.includes("google") || name.includes("imagen-") || name.includes("veo-")) return providerIcon("gemini");
    if (isSeedanceVideoAlias(name) || name.includes("doubao") || name.includes("seedance") || name.includes("seedream") || name.includes("volcengine") || /(^|[-_:/.])ark(?:[-_:/.]|$)/.test(name)) return providerIcon("doubao");
    if (name.includes("minimax") || name.includes("abab") || name.includes("hailuo")) return providerIcon("minimax");
    if (name.startsWith("gpt") || name.startsWith("o1") || name.startsWith("o3") || name.startsWith("o4") || name.includes("openai") || name.includes("chatgpt") || name.includes("dall-e") || name.includes("whisper") || name.includes("tts-1") || name.includes("text-embedding-3") || name.includes("text-moderation") || name.includes("sora")) return "openai";
    if (name.includes("grok") || name.includes("xai")) return providerIcon("grok");
    if (name.includes("deepseek")) return providerIcon("deepseek");
    if (name.includes("glm") || name.includes("chatglm") || name.includes("cogview") || name.includes("cogvideo") || name.includes("zhipu")) return providerIcon("glm");
    if (name.includes("qwen") || name.includes("qwq")) return providerIcon("qwen");
    if (name.includes("mistral") || name.includes("mixtral") || name.includes("codestral") || name.includes("pixtral") || name.includes("voxtral") || name.includes("magistral")) return providerIcon("mistral");
    if (name.includes("llama") || name.includes("meta")) return providerIcon("meta");
    if (platform.includes("anthropic")) return providerIcon("claude");
    if (platform.includes("gemini") || platform.includes("google")) return providerIcon("gemini");
    if (platform.includes("grok") || platform.includes("xai")) return providerIcon("grok");
    if (platform.includes("zhipu") || platform.includes("glm")) return providerIcon("glm");
    if (platform.includes("doubao") || platform.includes("volcengine") || platform.includes("ark") || platform.includes("bytedance")) return providerIcon("doubao");
    if (platform.includes("minimax")) return providerIcon("minimax");
    if (platform.includes("deepseek")) return providerIcon("deepseek");
    if (platform.includes("qwen") || platform.includes("alibaba")) return providerIcon("qwen");
    if (platform.includes("mistral")) return providerIcon("mistral");
    if (platform.includes("meta")) return providerIcon("meta");
    if (platform.includes("openai")) return "openai";
    return "";
}

function providerIcon(name: string) {
    return `${import.meta.env.BASE_URL}icons/${name}.svg`;
}
