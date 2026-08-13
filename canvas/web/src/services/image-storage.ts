import localforage from "localforage";

import { nanoid } from "nanoid";
import i18n from "@/i18n";
import { dataUrlToBlob, readImageMeta } from "@/lib/image-utils";

export type UploadedImage = {
    url: string;
    storageKey: string;
    thumbnailUrl?: string;
    thumbnailStorageKey?: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
};

const THUMBNAIL_MAX_EDGE = 768;
const THUMBNAIL_QUALITY = 0.82;

const store = localforage.createInstance({ name: "infinite-canvas", storeName: "image_files" });
const imageLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "image_generation_logs" });
const videoLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "video_generation_logs" });
const objectUrls = new Map<string, string>();

export async function uploadImage(input: string | Blob): Promise<UploadedImage> {
    const blob = typeof input === "string" ? (input.startsWith("data:") ? dataUrlToBlob(input) : await (await fetch(input)).blob()) : input;
    const storageKey = `image:${nanoid()}`;
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    const meta = await readImageMeta(url);
    const thumbnail = await createImageThumbnail(blob, meta.width, meta.height);
    let thumbnailUrl: string | undefined;
    let thumbnailStorageKey: string | undefined;
    if (thumbnail) {
        thumbnailStorageKey = `${storageKey}:thumb`;
        await store.setItem(thumbnailStorageKey, thumbnail);
        thumbnailUrl = URL.createObjectURL(thumbnail);
        objectUrls.set(thumbnailStorageKey, thumbnailUrl);
    }
    return { url, storageKey, thumbnailUrl, thumbnailStorageKey, width: meta.width, height: meta.height, bytes: blob.size, mimeType: blob.type || meta.mimeType };
}

export async function resolveImageUrl(storageKey?: string, fallback = "") {
    if (!storageKey) return fallback;
    const cached = objectUrls.get(storageKey);
    if (cached) return cached;
    const blob = await store.getItem<Blob>(storageKey);
    if (!blob) return fallback;
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function resolveImageThumbnailUrl(thumbnailStorageKey?: string, fallback = "", sourceStorageKey?: string) {
    const derivedKey = thumbnailStorageKey || (sourceStorageKey ? `${sourceStorageKey}:thumb` : undefined);
    if (derivedKey) {
        const cached = objectUrls.get(derivedKey);
        if (cached) return cached;
        const blob = await store.getItem<Blob>(derivedKey);
        if (blob) {
            const url = URL.createObjectURL(blob);
            objectUrls.set(derivedKey, url);
            return url;
        }
    }

    if (!sourceStorageKey) return fallback;
    const sourceBlob = await store.getItem<Blob>(sourceStorageKey);
    if (!sourceBlob) return fallback;
    const sourceUrl = await resolveImageUrl(sourceStorageKey, "");
    if (!sourceUrl) return fallback;
    const meta = await readImageMeta(sourceUrl);
    const thumbnail = await createImageThumbnail(sourceBlob, meta.width, meta.height);
    if (!thumbnail) return fallback || sourceUrl;
    const key = derivedKey || `${sourceStorageKey}:thumb`;
    await store.setItem(key, thumbnail);
    const url = URL.createObjectURL(thumbnail);
    objectUrls.set(key, url);
    return url;
}

export async function getImageBlob(storageKey: string) {
    return store.getItem<Blob>(storageKey);
}

export async function setImageBlob(storageKey: string, blob: Blob) {
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function imageToDataUrl(image: { url?: string; dataUrl?: string; storageKey?: string }) {
    if (image.dataUrl?.startsWith("data:")) return image.dataUrl;
    if (image.storageKey) {
        const blob = await getImageBlob(image.storageKey);
        if (blob) return blobToDataUrl(blob);
    }
    const url = image.dataUrl || image.url || "";
    if (!url || url.startsWith("data:")) return url;
    if (url.startsWith("blob:")) throw new Error(i18n.t("common.imageReadFailed"));
    return blobToDataUrl(await (await fetch(url)).blob());
}

export async function deleteStoredImages(keys: Iterable<string>) {
    const expandedKeys = Array.from(new Set(Array.from(keys).flatMap((key) => (key.endsWith(":thumb") ? [key] : [key, `${key}:thumb`]))));
    await Promise.all(
        expandedKeys.map(async (key) => {
            const url = objectUrls.get(key);
            if (url) URL.revokeObjectURL(url);
            objectUrls.delete(key);
            await store.removeItem(key);
        }),
    );
}

export async function cleanupUnusedImages(usedData: unknown) {
    const usedKeys = collectImageStorageKeys(usedData);
    await Promise.all([
        imageLogStore.iterate((value) => {
            collectImageStorageKeys(value, usedKeys);
        }),
        videoLogStore.iterate((value) => {
            collectImageStorageKeys(value, usedKeys);
        }),
    ]);
    const unused: string[] = [];
    await store.iterate((_value, key) => {
        if (!usedKeys.has(key)) unused.push(key);
    });
    await deleteStoredImages(unused);
}

export function collectImageStorageKeys(value: unknown, keys = new Set<string>()) {
    if (!value || typeof value !== "object") return keys;
    if ("storageKey" in value && typeof value.storageKey === "string" && value.storageKey.startsWith("image:")) {
        keys.add(value.storageKey);
        keys.add(`${value.storageKey}:thumb`);
    }
    if ("thumbnailStorageKey" in value && typeof value.thumbnailStorageKey === "string" && value.thumbnailStorageKey.startsWith("image:")) keys.add(value.thumbnailStorageKey);
    Object.values(value).forEach((item) => (Array.isArray(item) ? item.forEach((child) => collectImageStorageKeys(child, keys)) : collectImageStorageKeys(item, keys)));
    return keys;
}

async function createImageThumbnail(blob: Blob, width: number, height: number) {
    if (typeof document === "undefined" || !width || !height) return null;
    const scale = Math.min(1, THUMBNAIL_MAX_EDGE / Math.max(width, height));
    const targetWidth = Math.max(1, Math.round(width * scale));
    const targetHeight = Math.max(1, Math.round(height * scale));
    const sourceUrl = URL.createObjectURL(blob);
    try {
        const image = await new Promise<HTMLImageElement>((resolve, reject) => {
            const element = new Image();
            element.onload = () => resolve(element);
            element.onerror = () => reject(new Error("Image thumbnail decode failed"));
            element.src = sourceUrl;
        });
        const canvas = document.createElement("canvas");
        canvas.width = targetWidth;
        canvas.height = targetHeight;
        const context = canvas.getContext("2d");
        if (!context) return null;
        context.drawImage(image, 0, 0, targetWidth, targetHeight);
        const webp = await canvasToBlob(canvas, "image/webp", THUMBNAIL_QUALITY);
        if (webp) return webp;
        return canvasToBlob(canvas, "image/jpeg", THUMBNAIL_QUALITY);
    } catch {
        return null;
    } finally {
        URL.revokeObjectURL(sourceUrl);
    }
}

function canvasToBlob(canvas: HTMLCanvasElement, type: string, quality: number) {
    return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, type, quality));
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error(i18n.t("common.imageReadFailed")));
        reader.readAsDataURL(blob);
    });
}
