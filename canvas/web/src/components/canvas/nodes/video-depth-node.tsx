import { useEffect, useRef, useState } from "react";
import { Button, Progress, Segmented } from "antd";
import { nanoid } from "nanoid";
import { deleteStoredMedia, getMediaBlob, uploadMediaFile } from "@/services/file-storage";
import { withLocalProxy } from "@/stores/use-config-store";
import { CanvasNodeType } from "@/types/canvas";
import type { CanvasNodeContext, CanvasNodeDefinition } from "@/types/canvas-plugin";
import { videoMetadata } from "@/lib/canvas/canvas-node-factory";
import { Video } from "lucide-react";

export const VIDEO_DEPTH_NODE_TYPE = "canvas:video-depth";

type DepthProgress = {
    label: string;
    percent?: number;
    stats?: { backend: string; frames: number; inferred: number; fps: number; elapsed: number; remaining?: number };
};

function formatDuration(seconds: number) {
    const rounded = Math.max(0, Math.ceil(seconds));
    return `${Math.floor(rounded / 60)} 分 ${rounded % 60} 秒`;
}

function VideoDepthNode({ ctx }: { ctx: CanvasNodeContext }) {
    const contextRef = useRef(ctx);
    contextRef.current = ctx;
    const workerRef = useRef<Worker | null>(null);
    const abortRef = useRef<AbortController | null>(null);
    const [running, setRunning] = useState(false);
    const [progress, setProgress] = useState<DepthProgress>({ label: "" });
    const error = ctx.node.metadata?.errorDetails || "";
    const mode = ctx.node.metadata?.depthMode || "auto";
    const samplingFps = ctx.node.metadata?.depthSamplingFps ?? 10;
    const sourceNode = ctx.getUpstream().find((item) => item.type === CanvasNodeType.Video && item.metadata?.content);

    const cancel = () => {
        abortRef.current?.abort();
        workerRef.current?.terminate();
        setRunning(false);
        setProgress({ label: "已取消" });
    };

    useEffect(() => () => {
        abortRef.current?.abort();
        workerRef.current?.terminate();
    }, []);

    const start = async () => {
        if (abortRef.current) return;
        const node = sourceNode;
        if (!node?.metadata?.content) return;
        const controller = new AbortController();
        abortRef.current = controller;
        setRunning(true);
        ctx.updateMetadata({ errorDetails: undefined });
        setProgress({ label: "正在读取源视频" });
        try {
            let blob = node.metadata?.storageKey ? await getMediaBlob(node.metadata.storageKey) : null;
            if (!blob) {
                const response = await fetch(withLocalProxy(node.metadata!.content!), { signal: controller.signal });
                if (!response.ok) throw new Error(`读取视频失败 (${response.status})`);
                blob = await response.blob();
            }
            if (controller.signal.aborted) return;
            const worker = new Worker(new URL("../../../lib/canvas/video-depth.worker.ts", import.meta.url), { type: "module" });
            workerRef.current = worker;
            const result = await new Promise<Blob>((resolve, reject) => {
                controller.signal.addEventListener("abort", () => reject(new DOMException("已取消", "AbortError")), { once: true });
                worker.onerror = () => reject(new Error("深度处理 Worker 启动失败，请检查浏览器安全策略及 WebAssembly 支持。"));
                worker.onmessage = ({ data }) => {
                    if (data.type === "progress") setProgress({ label: data.label, percent: data.percent, stats: data.stats });
                    else if (data.type === "complete") resolve(data.blob);
                    else if (data.type === "error") reject(new Error(data.message));
                };
                const base = new URL(import.meta.env.BASE_URL, window.location.href);
                worker.postMessage({ blob, mode, samplingFps, wasm: { mjs: new URL("depth-runtime/ort-wasm-simd-threaded.jsep.mjs", base).href, wasm: new URL("depth-runtime/ort-wasm-simd-threaded.jsep.wasm", base).href } });
            });
            worker.terminate();
            workerRef.current = null;
            if (controller.signal.aborted) return;
            setProgress({ label: "正在保存深度视频", percent: 100 });
            const video = await uploadMediaFile(result, "video-depth");
            if (controller.signal.aborted) {
                await deleteStoredMedia([video.storageKey]);
                return;
            }
            const current = contextRef.current;
            const resultId = nanoid();
            const resultSize = { width: node.width, height: node.height };
            const x = current.node.position.x + current.node.width + 96;
            let y = current.node.position.y;
            while (current.getNodes().some((item) => item.position.x < x + resultSize.width && item.position.x + item.width > x && item.position.y < y + resultSize.height && item.position.y + item.height > y)) y += resultSize.height + 24;
            current.applyOps([
                { type: "add_node", id: resultId, nodeType: CanvasNodeType.Video, title: `${node.title || "视频"} · 灰度深度`, position: { x, y }, ...resultSize, metadata: videoMetadata(video) },
                { type: "connect_nodes", fromNodeId: current.node.id, toNodeId: resultId },
            ]);
            setProgress({ label: "已完成", percent: 100 });
        } catch (cause) {
            if (!controller.signal.aborted) contextRef.current.updateMetadata({ errorDetails: cause instanceof Error ? cause.message : "深度视频转换失败" });
        } finally {
            workerRef.current?.terminate();
            workerRef.current = null;
            abortRef.current = null;
            if (!controller.signal.aborted) setRunning(false);
        }
    };

    return (
        <div
            className="h-full w-full overflow-auto p-4 text-sm"
            style={{ color: ctx.theme.node.text, background: ctx.theme.node.panel }}
            onPointerDown={(event) => {
                if ((event.target as HTMLElement).closest(".ant-segmented, button")) event.stopPropagation();
            }}
            onMouseDown={(event) => {
                if ((event.target as HTMLElement).closest(".ant-segmented, button")) event.stopPropagation();
            }}
            onWheel={(event) => event.stopPropagation()}
        >
            <div className="space-y-3">
                <div className="truncate text-xs opacity-60">{sourceNode?.title || "请连接源视频"}</div>
                <Segmented block disabled={running} value={mode} options={[{ label: "GPU 优先", value: "auto" }, { label: "CPU", value: "cpu" }]} onChange={(value) => ctx.updateMetadata({ depthMode: value as "auto" | "cpu" })} />
                <div className="space-y-2">
                    <div className="text-xs opacity-60">采样频率</div>
                    <Segmented block disabled={running} value={samplingFps} options={[{ label: "5 FPS", value: 5 }, { label: "10 FPS", value: 10 }, { label: "15 FPS", value: 15 }, { label: "逐帧", value: 0 }]} onChange={(value) => ctx.updateMetadata({ depthSamplingFps: Number(value) })} />
                </div>
                {running ? <div role="status" className="space-y-2">
                    <p>{progress.label}</p>
                    <Progress percent={progress.percent ?? 0} status="active" showInfo={progress.percent !== undefined} />
                    {progress.stats ? <div className="space-y-1 text-sm">
                        <p>{progress.stats.backend} · {progress.stats.fps.toFixed(1)} 帧/秒</p>
                        <p>输出 {progress.stats.frames} 帧 · 推理 {progress.stats.inferred} 帧</p>
                        <p>预计剩余：{progress.stats.remaining === undefined ? "正在估算…" : formatDuration(progress.stats.remaining)}</p>
                        {progress.stats.remaining !== undefined ? <p>预计总计：{formatDuration(progress.stats.elapsed + progress.stats.remaining)}</p> : null}
                    </div> : <p className="text-xs opacity-60">{(progress.percent ?? 0) >= 99 ? "正在完成…" : "预计时间：估算中…"}</p>}
                </div> : null}
                {!running && progress.label ? <p className="text-xs opacity-60">{progress.label}</p> : null}
                {error ? <p role="alert" className="text-red-500">{error}</p> : null}
                <div className="flex justify-end gap-2">
                    {running ? <Button type="text" onClick={cancel}>取消</Button> : <Button type="text" disabled={!sourceNode} onClick={() => void start()}>开始转换</Button>}
                </div>
            </div>
        </div>
    );
}

export const videoDepthNodeDefinition: CanvasNodeDefinition = {
    type: VIDEO_DEPTH_NODE_TYPE,
    title: "深度视频",
    icon: <Video className="size-5" />,
    defaultSize: { width: 400, height: 380 },
    defaultMetadata: { depthMode: "auto", depthSamplingFps: 10 },
    hidePanel: true,
    showInCreateMenu: false,
    Content: VideoDepthNode,
};
