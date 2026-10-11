import { modelOptionName, type AiConfig } from "@/stores/use-config-store";

export const xiaowenResolutionOptions = [{ value: "768p", label: "768p" }, { value: "2K", label: "2K" }];

export function isXiaowenVideoModel(model: string) {
    return /^xiaowen(?:$|[-_:])/i.test(modelOptionName(model).trim().split("/").pop() || "");
}

export function isXiaowenVideoConfig(config: Pick<AiConfig, "model" | "videoModel">) {
    return isXiaowenVideoModel(config.model || config.videoModel);
}

export function normalizeXiaowenResolution(value: string) {
    return value.trim().toLowerCase() === "2k" ? "2K" : "768p";
}
