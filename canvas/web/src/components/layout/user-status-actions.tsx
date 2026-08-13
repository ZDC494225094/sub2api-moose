import type { CSSProperties, ReactNode } from "react";
import { useEffect, useRef, useState } from "react";
import { Tooltip } from "antd";
import { BookOpen, ChevronDown, CreditCard, KeyRound, Keyboard, LogOut, Puzzle, Settings2, Shield, UserRound, Wallet } from "lucide-react";
import { useTranslation } from "react-i18next";

import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { GitHubLink } from "@/components/layout/github-link";
import { VersionReleaseModal } from "@/components/layout/version-release-modal";
import { DOCS_URL } from "@/constant/env";
import { changeAppLocale, type AppLocale } from "@/i18n";
import { cn } from "@/lib/utils";
import { canvasThemes } from "@/lib/canvas-theme";
import { useConfigStore } from "@/stores/use-config-store";
import { useThemeStore } from "@/stores/use-theme-store";
import { useUserStore, type LocalSubscription } from "@/stores/use-user-store";

type UserStatusActionsProps = {
    showConfig?: boolean;
    variant?: "default" | "canvas";
    onOpenShortcuts?: () => void;
    onOpenPlugins?: () => void;
};

export function UserStatusActions({ showConfig = true, variant = "default", onOpenShortcuts, onOpenPlugins }: UserStatusActionsProps) {
    const { i18n, t } = useTranslation();
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);
    const user = useUserStore((state) => state.user);
    const clearSession = useUserStore((state) => state.clearSession);
    const [menuOpen, setMenuOpen] = useState(false);
    const menuRef = useRef<HTMLDivElement>(null);
    const canvasTheme = canvasThemes[theme];
    const naturalIconClass = "inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-stone-600 transition-colors hover:bg-black/5 hover:text-stone-950 dark:text-stone-300 dark:hover:bg-white/10 dark:hover:text-white [&_svg]:size-4";
    const iconStyle: CSSProperties | undefined = variant === "canvas" ? { color: canvasTheme.node.text } : undefined;
    const locale = i18n.resolvedLanguage as AppLocale;
    const nextLocale = locale === "zh-CN" ? "en-US" : "zh-CN";
    const languageLabel = t("topNav.switchLanguage", { language: t(nextLocale === "zh-CN" ? "locale.zhCN" : "locale.enUS") });
    const isAdmin = user?.role === "admin";
    const displayName = user?.displayName || user?.username || user?.email || "";
    const initials = displayName.slice(0, 2).toUpperCase();
    const subscriptions = user?.subscriptions || [];

    useEffect(() => {
        const closeMenu = (event: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(event.target as Node)) setMenuOpen(false);
        };
        document.addEventListener("mousedown", closeMenu);
        return () => document.removeEventListener("mousedown", closeMenu);
    }, []);

    const handleLogout = () => {
        const refreshToken = localStorage.getItem("refresh_token");
        const token = localStorage.getItem("auth_token");
        if (refreshToken) {
            void fetch("/api/v1/auth/logout", {
                method: "POST",
                keepalive: true,
                headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
                body: JSON.stringify({ refresh_token: refreshToken }),
            });
        }
        localStorage.removeItem("auth_token");
        localStorage.removeItem("refresh_token");
        localStorage.removeItem("token_expires_at");
        localStorage.removeItem("auth_user");
        clearSession();
        window.location.assign("/login");
    };

    return (
        <div className="inline-flex shrink-0 items-center gap-1">
            {onOpenPlugins ? (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={onOpenPlugins} aria-label={t("topNav.plugins")} title={t("topNav.plugins")}>
                    <Puzzle className="size-4" />
                </button>
            ) : null}
            {isAdmin ? (
                <a href={DOCS_URL} target="_blank" rel="noopener noreferrer" className={naturalIconClass} style={iconStyle} aria-label={t("topNav.docs")} title={t("topNav.docs")}>
                    <BookOpen className="size-4" />
                </a>
            ) : null}
            {showConfig ? (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={() => openConfigDialog(false)} aria-label={t("navigation.config")} title={t("navigation.config")}>
                    <Settings2 className="size-4" />
                </button>
            ) : null}
            <Tooltip title={languageLabel} mouseEnterDelay={0.2}>
                <button type="button" className={`${naturalIconClass} text-[11px] font-semibold tracking-tight`} style={iconStyle} onClick={() => void changeAppLocale(nextLocale)} aria-label={languageLabel}>
                    {locale === "zh-CN" ? "中" : "EN"}
                </button>
            </Tooltip>
            <AnimatedThemeToggler theme={theme} onThemeChange={setTheme} className={naturalIconClass} style={iconStyle} aria-label={t(theme === "dark" ? "topNav.lightTheme" : "topNav.darkTheme")} title={t(theme === "dark" ? "topNav.lightTheme" : "topNav.darkTheme")} />
            {isAdmin ? <VersionReleaseModal style={iconStyle} /> : null}
            {isAdmin ? <GitHubLink className={cn("bg-transparent hover:bg-transparent dark:hover:bg-transparent", "size-7 text-base")} style={iconStyle} /> : null}
            {onOpenShortcuts ? (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={onOpenShortcuts} aria-label={t("topNav.shortcuts")} title={t("topNav.shortcuts")}>
                    <Keyboard className="size-4" />
                </button>
            ) : null}
            {user ? (
                <>
                    <SubscriptionStatus subscriptions={subscriptions} label={t("topNav.subscriptions")} />
                    <div className="group relative hidden h-8 items-center gap-1.5 rounded-xl bg-sky-50 px-2.5 text-xs font-semibold text-sky-700 transition-colors hover:bg-sky-100 sm:flex dark:bg-sky-950/30 dark:text-sky-300 dark:hover:bg-sky-900/40" title={`${t("topNav.balance")}: ${formatMoney(user.balance)}${user.frozenBalance > 0 ? ` | ${t("topNav.frozen")}: ${formatMoney(user.frozenBalance)}` : ""}`}>
                        <Wallet className="size-3.5" />
                        <span>{formatMoney(user.balance)}</span>
                        {user.frozenBalance > 0 ? <span className="rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] leading-none text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">{t("topNav.frozen")} {formatMoney(user.frozenBalance)}</span> : null}
                        <div className="pointer-events-auto invisible absolute right-0 top-full z-50 mt-2 w-56 translate-y-1 rounded-lg border border-stone-200 bg-background p-3 text-xs font-normal text-stone-600 opacity-0 shadow-xl transition-all duration-150 group-hover:visible group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:visible group-focus-within:translate-y-0 group-focus-within:opacity-100 dark:border-stone-700 dark:text-stone-300">
                            <StatusRow label={t("topNav.balance")} value={formatMoney(user.balance)} />
                            <StatusRow label={t("topNav.frozen")} value={formatMoney(user.frozenBalance)} muted={!user.frozenBalance} />
                            <div className="my-2 border-t border-stone-100 dark:border-stone-800" />
                            <StatusRow label="总余额" value={formatMoney(user.balance + user.frozenBalance)} strong />
                        </div>
                    </div>
                    <div ref={menuRef} className="relative">
                        <button type="button" className="flex h-9 max-w-40 items-center gap-2 rounded-lg px-1.5 transition-colors hover:bg-black/5 dark:hover:bg-white/10" onClick={() => setMenuOpen((open) => !open)} aria-label={t("topNav.userMenu")} aria-expanded={menuOpen}>
                            <span className="flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-sky-600 text-xs font-semibold text-white">
                                {user.avatarUrl ? <img src={user.avatarUrl} alt={displayName} className="h-full w-full object-cover" /> : initials}
                            </span>
                            <span className="hidden min-w-0 text-left md:block">
                                <span className="block truncate text-sm font-medium text-stone-900 dark:text-stone-100">{displayName}</span>
                            </span>
                            <ChevronDown className={`hidden size-3.5 shrink-0 text-stone-400 transition-transform md:block ${menuOpen ? "rotate-180" : ""}`} />
                        </button>
                        {menuOpen ? (
                            <div className="absolute right-0 top-full z-50 mt-2 w-56 overflow-hidden rounded-lg border border-stone-200 bg-background py-1 shadow-xl dark:border-stone-700">
                                <div className="border-b border-stone-100 px-3 py-2.5 dark:border-stone-800">
                                    <div className="truncate text-sm font-medium text-stone-900 dark:text-stone-100">{displayName}</div>
                                    {user.email ? <div className="mt-0.5 truncate text-xs text-stone-500 dark:text-stone-400">{user.email}</div> : null}
                                </div>
                                <div className="border-b border-stone-100 px-3 py-2 sm:hidden dark:border-stone-800">
                                    <div className="text-xs text-stone-500 dark:text-stone-400">{t("topNav.balance")}</div>
                                    <div className="mt-0.5 text-sm font-semibold text-sky-700 dark:text-sky-300">{formatMoney(user.balance)}</div>
                                    {user.frozenBalance > 0 ? <div className="mt-1 text-xs text-amber-700 dark:text-amber-300">{t("topNav.frozen")} {formatMoney(user.frozenBalance)}</div> : null}
                                </div>
                                <div className="py-1">
                                    <TopNavMenuLink href="/profile" icon={<UserRound className="size-4" />} label={t("topNav.profile")} onClick={() => setMenuOpen(false)} />
                                    <TopNavMenuLink href="/keys" icon={<KeyRound className="size-4" />} label={t("topNav.apiKeys")} onClick={() => setMenuOpen(false)} />
                                    <TopNavMenuLink href="/subscriptions" icon={<CreditCard className="size-4" />} label={t("topNav.subscriptions")} onClick={() => setMenuOpen(false)} />
                                    {isAdmin ? <TopNavMenuLink href="/admin" icon={<Shield className="size-4" />} label={t("topNav.admin")} onClick={() => setMenuOpen(false)} /> : null}
                                </div>
                                <div className="border-t border-stone-100 py-1 dark:border-stone-800">
                                    <button type="button" className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/20" onClick={handleLogout}>
                                        <LogOut className="size-4" />
                                        {t("topNav.logout")}
                                    </button>
                                </div>
                            </div>
                        ) : null}
                    </div>
                </>
            ) : null}
        </div>
    );
}

function TopNavMenuLink({ href, icon, label, onClick }: { href: string; icon: ReactNode; label: string; onClick: () => void }) {
    return (
        <a href={href} className="flex items-center gap-2 px-3 py-2 text-sm text-stone-700 transition-colors hover:bg-stone-100 dark:text-stone-200 dark:hover:bg-stone-800" onClick={onClick}>
            {icon}
            <span>{label}</span>
        </a>
    );
}

function SubscriptionStatus({ subscriptions, label }: { subscriptions: LocalSubscription[]; label: string }) {
    if (!subscriptions.length) return null;
    return (
        <div className="group relative hidden lg:block">
            <a href="/subscriptions" className="flex h-8 items-center gap-2 rounded-xl bg-violet-50 px-2.5 text-xs font-medium text-violet-700 transition-colors hover:bg-violet-100 dark:bg-violet-950/25 dark:text-violet-300 dark:hover:bg-violet-900/35" title={label}>
                <CreditCard className="size-3.5 shrink-0" />
                <span className="flex items-center gap-1">
                    {subscriptions.slice(0, 3).map((subscription) => <span key={subscription.id} className={cn("size-2 rounded-full", subscriptionDotClass(subscription))} />)}
                </span>
                <span className="tabular-nums">{subscriptions.length}</span>
            </a>
            <div className="pointer-events-auto invisible absolute right-0 top-full z-50 mt-2 w-80 translate-y-1 overflow-hidden rounded-xl border border-stone-200 bg-background opacity-0 shadow-xl transition-all duration-150 group-hover:visible group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:visible group-focus-within:translate-y-0 group-focus-within:opacity-100 dark:border-stone-700">
                <div className="border-b border-stone-100 px-3 py-2.5 dark:border-stone-800">
                    <div className="text-sm font-semibold text-stone-900 dark:text-stone-100">{label}</div>
                    <div className="mt-0.5 text-xs text-stone-500 dark:text-stone-400">{subscriptions.length} 个生效订阅</div>
                </div>
                <div className="max-h-72 overflow-y-auto">
                    {subscriptions.map((subscription) => <SubscriptionQuota key={subscription.id} subscription={subscription} />)}
                </div>
            </div>
        </div>
    );
}

function SubscriptionQuota({ subscription }: { subscription: LocalSubscription }) {
    const quotas = [
        { label: "日剩余", used: subscription.dailyUsage, limit: subscription.dailyLimit },
        { label: "周剩余", used: subscription.weeklyUsage, limit: subscription.weeklyLimit },
        { label: "月剩余", used: subscription.monthlyUsage, limit: subscription.monthlyLimit },
    ].filter((quota): quota is { label: string; used: number; limit: number } => quota.limit !== null && quota.limit > 0);

    return (
        <div className="border-b border-stone-100 px-3 py-2.5 last:border-b-0 dark:border-stone-800">
            <div className="mb-2 flex items-center justify-between gap-3">
                <span className="truncate text-sm font-medium text-stone-900 dark:text-stone-100">{subscription.groupName}</span>
                {subscription.expiresAt ? <span className="shrink-0 text-xs text-stone-500 dark:text-stone-400">{remainingDays(subscription.expiresAt)}</span> : null}
            </div>
            {quotas.length ? (
                <div className="space-y-1.5">
                    {quotas.map((quota) => <QuotaRow key={quota.label} {...quota} />)}
                </div>
            ) : <div className="text-xs text-emerald-600 dark:text-emerald-400">不限额度</div>}
        </div>
    );
}

function QuotaRow({ label, used, limit }: { label: string; used: number; limit: number }) {
    const remaining = Math.max(limit - used, 0);
    const percentage = Math.min((used / limit) * 100, 100);
    return (
        <div className="grid grid-cols-[2.75rem_minmax(0,1fr)_5rem] items-center gap-2 text-[11px]">
            <span className="text-stone-500 dark:text-stone-400">{label}</span>
            <span className="h-1.5 overflow-hidden rounded-full bg-stone-100 dark:bg-stone-800"><span className={cn("block h-full rounded-full", percentage >= 90 ? "bg-red-500" : percentage >= 70 ? "bg-amber-500" : "bg-emerald-500")} style={{ width: `${percentage}%` }} /></span>
            <span className="text-right tabular-nums text-stone-600 dark:text-stone-300">{formatMoney(remaining)}</span>
        </div>
    );
}

function StatusRow({ label, value, strong = false, muted = false }: { label: string; value: string; strong?: boolean; muted?: boolean }) {
    return <div className={cn("flex items-center justify-between gap-3", muted && "opacity-60")}><span>{label}</span><span className={cn("tabular-nums text-stone-900 dark:text-stone-100", strong && "font-semibold")}>{value}</span></div>;
}

function subscriptionDotClass(subscription: LocalSubscription) {
    const usages = [subscription.dailyUsage, subscription.weeklyUsage, subscription.monthlyUsage];
    const ratios = [subscription.dailyLimit, subscription.weeklyLimit, subscription.monthlyLimit]
        .map((limit, index) => limit !== null && limit > 0 ? usages[index]! / limit : 0);
    const percentage = Math.max(...ratios, 0) * 100;
    if (!ratios.some(Boolean)) return "bg-emerald-500";
    if (percentage >= 90) return "bg-red-500";
    if (percentage >= 70) return "bg-amber-500";
    return "bg-emerald-500";
}

function remainingDays(expiresAt: string) {
    const expires = new Date(expiresAt).getTime();
    if (!Number.isFinite(expires)) return "";
    const days = Math.ceil((expires - Date.now()) / 86_400_000);
    return days <= 0 ? "已到期" : `${days} 天后到期`;
}

function formatMoney(value: number) {
    return `$${Number.isFinite(value) ? value.toFixed(2) : "0.00"}`;
}
