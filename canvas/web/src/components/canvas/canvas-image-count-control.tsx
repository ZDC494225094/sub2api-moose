import { Minus, Plus } from "lucide-react";
import { Button, Tooltip } from "antd";
import { useTranslation } from "react-i18next";

import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";

type CanvasImageCountControlProps = {
    count: string;
    onChange: (count: number) => void;
    className?: string;
};

export function CanvasImageCountControl({ count, onChange, className = "!w-[78px] !rounded-full" }: CanvasImageCountControlProps) {
    const { t } = useTranslation();
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const value = Math.max(1, Math.min(15, Math.floor(Math.abs(Number(count)) || 1)));
    const label = t("settingsPanels.image.count");

    return (
        <div className={`flex h-10 shrink-0 items-center justify-between border px-0.5 ${className}`} style={{ background: theme.node.fill, borderColor: theme.node.stroke, color: theme.node.text }} role="group" aria-label={label} onMouseDown={(event) => event.stopPropagation()}>
            <Tooltip title={`${label} -`}>
                <Button type="text" size="small" className="!grid !h-8 !w-7 !min-w-7 !place-items-center !rounded-md !p-0" style={{ color: theme.node.text }} disabled={value === 1} icon={<Minus className="size-3.5" />} onClick={() => onChange(value - 1)} aria-label={`${label} -`} />
            </Tooltip>
            <span className="min-w-4 select-none text-center text-xs font-semibold tabular-nums">{value}</span>
            <Tooltip title={`${label} +`}>
                <Button type="text" size="small" className="!grid !h-8 !w-7 !min-w-7 !place-items-center !rounded-md !p-0" style={{ color: theme.node.text }} disabled={value === 15} icon={<Plus className="size-3.5" />} onClick={() => onChange(value + 1)} aria-label={`${label} +`} />
            </Tooltip>
        </div>
    );
}
