import { create } from "zustand";

export type LocalSubscription = {
    id: string;
    groupName: string;
    expiresAt: string;
    dailyUsage: number;
    weeklyUsage: number;
    monthlyUsage: number;
    dailyLimit: number | null;
    weeklyLimit: number | null;
    monthlyLimit: number | null;
};

export type LocalUser = {
    id: string;
    username: string;
    displayName: string;
    avatarUrl: string;
    email: string;
    role: string;
    balance: number;
    frozenBalance: number;
    subscriptions: LocalSubscription[];
};

type UserStore = {
    user: LocalUser | null;
    loadUser: () => Promise<void>;
    loadSubscriptions: () => Promise<void>;
    clearSession: () => void;
};

export const useUserStore = create<UserStore>()((set) => ({
    user: readCachedUser(),
    loadUser: async () => {
        if (typeof window === "undefined") return;
        const token = localStorage.getItem("auth_token");
        if (!token) {
            set({ user: null });
            return;
        }
        try {
            const response = await fetch("/api/v1/auth/me", { headers: { Authorization: `Bearer ${token}` } });
            if (!response.ok) {
                if (response.status === 401) set({ user: null });
                return;
            }
            const payload = (await response.json()) as { data?: unknown } | unknown;
            const user = normalizeUser(isObject(payload) && "data" in payload ? payload.data : payload, useUserStore.getState().user?.subscriptions);
            if (!user) return;
            localStorage.setItem("auth_user", JSON.stringify(user));
            set({ user });
        } catch {
            // Keep the latest cached profile visible when the refresh is temporarily unavailable.
        }
    },
    loadSubscriptions: async () => {
        if (typeof window === "undefined") return;
        const token = localStorage.getItem("auth_token");
        if (!token) return;
        try {
            const response = await fetch("/api/v1/subscriptions/active", { headers: { Authorization: `Bearer ${token}` } });
            if (!response.ok) return;
            const payload = (await response.json()) as { data?: unknown } | unknown;
            const value = isObject(payload) && "data" in payload ? payload.data : payload;
            if (!Array.isArray(value)) return;
            const subscriptions = value.map(normalizeSubscription).filter((subscription): subscription is LocalSubscription => Boolean(subscription));
            set((state) => (state.user ? { user: { ...state.user, subscriptions } } : {}));
        } catch {
            // Keep the latest subscription progress visible when the refresh is temporarily unavailable.
        }
    },
    clearSession: () => set({ user: null }),
}));

function readCachedUser() {
    if (typeof window === "undefined") return null;
    try {
        return normalizeUser(JSON.parse(localStorage.getItem("auth_user") || "null"));
    } catch {
        return null;
    }
}

function normalizeUser(value: unknown, previousSubscriptions: LocalSubscription[] = []): LocalUser | null {
    if (!isObject(value)) return null;
    const username = stringValue(value.username) || stringValue(value.email).split("@")[0];
    const subscriptions = Array.isArray(value.subscriptions)
        ? value.subscriptions.map(normalizeSubscription).filter((subscription): subscription is LocalSubscription => Boolean(subscription))
        : [];
    return {
        id: stringValue(value.id),
        username,
        displayName: username,
        avatarUrl: stringValue(value.avatar_url) || stringValue(value.avatarUrl),
        email: stringValue(value.email),
        role: stringValue(value.role),
        balance: numberValue(value.balance),
        frozenBalance: numberValue(value.frozen_balance ?? value.frozenBalance),
        subscriptions: subscriptions.length ? subscriptions : previousSubscriptions,
    };
}

function normalizeSubscription(value: unknown): LocalSubscription | null {
    if (!isObject(value)) return null;
    const group = isObject(value.group) ? value.group : {};
    return {
        id: stringValue(value.id),
        groupName: stringValue(group.name) || stringValue(value.group_name) || `Group #${stringValue(value.group_id)}`,
        expiresAt: stringValue(value.expires_at),
        dailyUsage: numberValue(value.daily_usage_usd),
        weeklyUsage: numberValue(value.weekly_usage_usd),
        monthlyUsage: numberValue(value.monthly_usage_usd),
        dailyLimit: nullableNumber(group.daily_limit_usd),
        weeklyLimit: nullableNumber(group.weekly_limit_usd),
        monthlyLimit: nullableNumber(group.monthly_limit_usd),
    };
}

function isObject(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object";
}

function stringValue(value: unknown) {
    return typeof value === "string" ? value : typeof value === "number" ? String(value) : "";
}

function numberValue(value: unknown) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
}

function nullableNumber(value: unknown) {
    if (value === null || value === undefined || value === "") return null;
    const number = Number(value);
    return Number.isFinite(number) ? number : null;
}
