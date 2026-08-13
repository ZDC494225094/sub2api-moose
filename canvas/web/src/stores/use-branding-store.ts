import { create } from "zustand";

const fallbackLogo = "/logo.svg";
const fallbackSiteName = "Sub2API";

type PublicBrandingSettings = {
    site_logo?: unknown;
    site_name?: unknown;
};

type BrandingStore = {
    logoUrl: string;
    siteName: string;
    loadBranding: () => Promise<void>;
};

let brandingRequest: Promise<void> | null = null;
const initialBranding = readInitialBranding();

export const useBrandingStore = create<BrandingStore>()((set) => ({
    logoUrl: initialBranding.logoUrl,
    siteName: initialBranding.siteName,
    loadBranding: async () => {
        if (typeof window === "undefined") return;
        if (!brandingRequest) {
            brandingRequest = fetch("/api/v1/settings/public")
                .then(async (response) => {
                    if (!response.ok) return;
                    const payload = (await response.json()) as { data?: unknown } | unknown;
                    const settings = isObject(payload) && "data" in payload ? payload.data : payload;
                    if (!isObject(settings)) return;
                    const logoUrl = normalizeLogoUrl(settings.site_logo);
                    const siteName = normalizeSiteName(settings.site_name);
                    if (logoUrl) updateFavicon(logoUrl);
                    set({ ...(logoUrl ? { logoUrl } : {}), ...(siteName ? { siteName } : {}) });
                })
                .catch(() => {
                    // Keep the bundled fallback when public settings are unavailable.
                })
                .finally(() => {
                    brandingRequest = null;
                });
        }
        await brandingRequest;
    },
}));

export function canvasTitle(siteName: string, productName: string) {
    return `${siteName || fallbackSiteName}-${productName}`;
}

export function getBrandingSiteName() {
    return useBrandingStore.getState().siteName || fallbackSiteName;
}

function readInitialBranding() {
    const settings = typeof window === "undefined" ? undefined : window.__APP_CONFIG__;
    const branding = isObject(settings) ? settings as PublicBrandingSettings : {};
    return {
        logoUrl: normalizeLogoUrl(branding.site_logo) || fallbackLogo,
        siteName: normalizeSiteName(branding.site_name) || fallbackSiteName,
    };
}

function normalizeLogoUrl(value: unknown) {
    if (typeof value !== "string") return "";
    const logoUrl = value.trim();
    if (/^data:image\/[a-z0-9.+-]+;base64,/i.test(logoUrl)) return logoUrl;
    if (/^(https?:)?\/\//i.test(logoUrl) || logoUrl.startsWith("/")) return logoUrl;
    return "";
}

function normalizeSiteName(value: unknown) {
    return typeof value === "string" ? value.trim() : "";
}

function isObject(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object";
}

declare global {
    interface Window {
        __APP_CONFIG__?: PublicBrandingSettings;
    }
}

function updateFavicon(logoUrl: string) {
    let favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!favicon) {
        favicon = document.createElement("link");
        favicon.rel = "icon";
        document.head.appendChild(favicon);
    }
    favicon.type = logoMimeType(logoUrl);
    favicon.href = logoUrl;
}

function logoMimeType(logoUrl: string) {
    const dataMimeType = logoUrl.match(/^data:(image\/[^;,]+)/i)?.[1];
    if (dataMimeType) return dataMimeType;
    if (/\.svg(?:[?#]|$)/i.test(logoUrl)) return "image/svg+xml";
    if (/\.png(?:[?#]|$)/i.test(logoUrl)) return "image/png";
    return "image/x-icon";
}
