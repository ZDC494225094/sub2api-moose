import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { Modal } from "antd";
import { useTranslation } from "react-i18next";

import { usePromptSourceScheduler } from "@/hooks/use-prompt-source-scheduler";
import { onInsufficientBalance } from "@/lib/canvas-billing";
import { bootstrapManagedCanvas } from "@/services/api/canvas";
import { useBrandingStore } from "@/stores/use-branding-store";
import { useUserStore } from "@/stores/use-user-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    usePromptSourceScheduler();
    const loadUser = useUserStore((state) => state.loadUser);
    const loadSubscriptions = useUserStore((state) => state.loadSubscriptions);
    const loadBranding = useBrandingStore((state) => state.loadBranding);

    useEffect(() => {
        const controller = new AbortController();
        if (!localStorage.getItem("auth_token")) {
            redirectToLogin();
            return () => controller.abort();
        }
        const refreshUser = () => void loadUser();
        const refreshSubscriptions = () => void loadSubscriptions();
        refreshUser();
        refreshSubscriptions();
        void loadBranding();
        void bootstrapManagedCanvas(controller.signal).catch((error: unknown) => {
            if (controller.signal.aborted) return;
            if (!(error instanceof DOMException && error.name === "AbortError")) console.error("Managed canvas bootstrap failed", error);
        });
        const userRefreshTimer = window.setInterval(refreshUser, 60_000);
        const subscriptionRefreshTimer = window.setInterval(refreshSubscriptions, 5 * 60_000);
        window.addEventListener("focus", refreshUser);
        window.addEventListener("focus", refreshSubscriptions);
        return () => {
            controller.abort();
            window.clearInterval(userRefreshTimer);
            window.clearInterval(subscriptionRefreshTimer);
            window.removeEventListener("focus", refreshUser);
            window.removeEventListener("focus", refreshSubscriptions);
        };
    }, [loadBranding, loadSubscriptions, loadUser]);

    return <>{children}<InsufficientBalanceDialog /></>;
}

function InsufficientBalanceDialog() {
    const { t } = useTranslation();
    const [message, setMessage] = useState("");

    useEffect(() => onInsufficientBalance(setMessage), []);

    return (
        <Modal
            open={Boolean(message)}
            centered
            title={t("billing.insufficientBalance.title")}
            okText={t("billing.insufficientBalance.recharge")}
            cancelText={t("common.cancel")}
            onCancel={() => setMessage("")}
            onOk={() => window.location.assign("/purchase")}
        >
            <p className="mb-0 text-sm leading-6 text-muted-foreground">{t("billing.insufficientBalance.description")}</p>
        </Modal>
    );
}

function redirectToLogin() {
    const redirect = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    window.location.replace(`/login?redirect=${encodeURIComponent(redirect)}`);
}
