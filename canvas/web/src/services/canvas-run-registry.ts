import type { CanvasConnection, CanvasNodeData, CanvasNodeImage } from "@/types/canvas";

const STORAGE_KEY = "infinite-canvas:pending-runs:v1";
const MAX_RECORD_AGE_MS = 7 * 24 * 60 * 60 * 1000;

export type PendingCanvasRun = {
    projectId: string;
    nodeId: string;
    imageId?: string;
    runId: string;
    node: CanvasNodeData;
    connections: CanvasConnection[];
    createdAt: number;
};

export function registerPendingCanvasRun(record: Omit<PendingCanvasRun, "createdAt">) {
    const records = readPendingCanvasRuns().filter((item) => item.runId !== record.runId);
    records.push({
        ...record,
        node: compactPendingNode(record.node),
        createdAt: Date.now(),
    });
    writePendingCanvasRuns(records);
}

export function removePendingCanvasRuns(runIds: Iterable<string>) {
    const ids = new Set(runIds);
    if (!ids.size) return;
    writePendingCanvasRuns(readPendingCanvasRuns().filter((record) => !ids.has(record.runId)));
}

export function completedPendingCanvasRuns(projectId: string, nodes: CanvasNodeData[]) {
    const nodeById = new Map(nodes.map((node) => [node.id, node]));
    return readPendingCanvasRuns().filter((record) => {
        if (record.projectId !== projectId) return false;
        const node = nodeById.get(record.nodeId);
        if (!node) return false;
        if (!record.imageId) return node.metadata?.status !== "loading";
        const image = node.metadata?.images?.find((item) => item.id === record.imageId);
        return image ? image.status !== "loading" : node.metadata?.status !== "loading";
    });
}

export function restorePendingCanvasRuns(projectId: string, nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    const records = readPendingCanvasRuns().filter((record) => record.projectId === projectId);
    if (!records.length) return { nodes, connections, records };

    const nodeById = new Map(nodes.map((node) => [node.id, node]));
    for (const record of records) {
        const current = nodeById.get(record.nodeId) || record.node;
        const currentImages = current.metadata?.images || [];
        const recordedImages = record.node.metadata?.images || [];
        const metadata = record.imageId
            ? {
                  ...current.metadata,
                  status: "loading" as const,
                  errorDetails: undefined,
                  images: mergePendingImages(currentImages, recordedImages, record.imageId, record.runId),
              }
            : { ...current.metadata, status: "loading" as const, generationRunId: record.runId, errorDetails: undefined };
        nodeById.set(record.nodeId, { ...current, metadata });
    }

    const connectionById = new Map(connections.map((connection) => [connection.id, connection]));
    records.flatMap((record) => record.connections).forEach((connection) => connectionById.set(connection.id, connection));
    return { nodes: Array.from(nodeById.values()), connections: Array.from(connectionById.values()), records };
}

function readPendingCanvasRuns(): PendingCanvasRun[] {
    try {
        const value = localStorage.getItem(STORAGE_KEY);
        if (!value) return [];
        const records = JSON.parse(value) as PendingCanvasRun[];
        const cutoff = Date.now() - MAX_RECORD_AGE_MS;
        return Array.isArray(records)
            ? records.filter((record) => record && typeof record.runId === "string" && typeof record.projectId === "string" && typeof record.nodeId === "string" && record.node && record.createdAt >= cutoff)
            : [];
    } catch {
        return [];
    }
}

function writePendingCanvasRuns(records: PendingCanvasRun[]) {
    try {
        if (records.length) localStorage.setItem(STORAGE_KEY, JSON.stringify(records));
        else localStorage.removeItem(STORAGE_KEY);
    } catch {
        // IndexedDB project persistence remains the fallback when localStorage is unavailable.
    }
}

function mergePendingImages(current: CanvasNodeImage[], recorded: CanvasNodeImage[], imageId: string, runId: string): CanvasNodeImage[] {
    const imageById = new Map((current || []).map((image) => [image.id, image]));
    for (const image of recorded || []) {
        if (!imageById.has(image.id)) imageById.set(image.id, image);
    }
    const pendingImage = imageById.get(imageId);
    if (pendingImage) {
        imageById.set(imageId, { ...pendingImage, status: "loading", generationRunId: runId, errorDetails: undefined });
    }
    return Array.from(imageById.values());
}

function compactPendingNode(node: CanvasNodeData): CanvasNodeData {
    if (!node.metadata) return node;
    return {
        ...node,
        metadata: {
            ...node.metadata,
            // Generated assets already live in IndexedDB. Keeping them out of
            // localStorage prevents large data/blob URLs from losing the run ID.
            content: node.metadata.status === "loading" ? undefined : node.metadata.content,
            previewContent: undefined,
            composerContent: undefined,
            references: undefined,
            images: node.metadata.images?.map((image) => ({
                ...image,
                content: "",
                previewContent: undefined,
            })),
        },
    };
}
