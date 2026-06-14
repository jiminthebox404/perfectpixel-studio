import { useEffect, useRef, useState } from "react";
import { Pause, Play, RotateCcw } from "lucide-react";
import { useI18n } from "../i18n";
import { Button } from "./ui/button";

interface IProps {
  frames: string[]; // dataURL 목록 (선택/정렬 반영)
  fps: number;
  loop: boolean;
  cellSize: number;
}

// 캔버스 기반 애니메이션 플레이어
export default function AnimPlayer({ frames, fps, loop, cellSize }: IProps) {
  const { t } = useI18n();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const imagesRef = useRef<HTMLImageElement[]>([]);
  const [playing, setPlaying] = useState(true);
  const [zoom, setZoom] = useState(1.5);
  const [playFps, setPlayFps] = useState(fps);
  const [frameIdx, setFrameIdx] = useState(0);
  const stateRef = useRef({ idx: 0, acc: 0, last: 0, playing: true, fps, loop });

  // 외부 fps 변경 반영
  useEffect(() => setPlayFps(fps), [fps]);

  useEffect(() => {
    stateRef.current.fps = playFps;
    stateRef.current.loop = loop;
    stateRef.current.playing = playing;
  }, [playFps, loop, playing]);

  // 프레임 이미지 로딩
  useEffect(() => {
    let cancelled = false;
    const imgs = frames.map((src) => {
      const img = new Image();
      img.src = src;
      return img;
    });
    Promise.all(
      imgs.map(
        (img) =>
          new Promise<void>((resolve) => {
            if (img.complete) resolve();
            else {
              img.onload = () => resolve();
              img.onerror = () => resolve();
            }
          })
      )
    ).then(() => {
      if (!cancelled) {
        imagesRef.current = imgs;
        stateRef.current.idx = 0;
        stateRef.current.acc = 0;
        setFrameIdx(0);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [frames]);

  // 렌더 루프
  useEffect(() => {
    let raf = 0;
    const tick = (t: number) => {
      raf = requestAnimationFrame(tick);
      const st = stateRef.current;
      const imgs = imagesRef.current;
      const canvas = canvasRef.current;
      if (!canvas || imgs.length === 0) return;

      if (st.last === 0) st.last = t;
      const dt = (t - st.last) / 1000;
      st.last = t;

      if (st.playing && imgs.length > 1) {
        st.acc += dt;
        const frameDur = 1 / Math.max(1, st.fps);
        while (st.acc >= frameDur) {
          st.acc -= frameDur;
          if (st.idx + 1 >= imgs.length) {
            if (st.loop) st.idx = 0;
            else st.acc = 0; // 마지막 프레임 유지
          } else {
            st.idx += 1;
          }
        }
        setFrameIdx(st.idx);
      }

      const img = imgs[Math.min(st.idx, imgs.length - 1)];
      if (!img || !img.naturalWidth) return;
      const w = img.naturalWidth;
      const h = img.naturalHeight;
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      const ctx = canvas.getContext("2d")!;
      ctx.imageSmoothingEnabled = false;
      ctx.clearRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, []);

  const scrub = (i: number) => {
    stateRef.current.idx = i;
    stateRef.current.acc = 0;
    setFrameIdx(i);
    setPlaying(false);
  };

  const replay = () => {
    stateRef.current.idx = 0;
    stateRef.current.acc = 0;
    setFrameIdx(0);
    setPlaying(true);
  };

  return (
    <>
      <div className="player-stage checker">
        <canvas
          ref={canvasRef}
          style={{
            width: cellSize * zoom,
            height: cellSize * zoom,
            maxWidth: "92%",
            maxHeight: "92%",
            objectFit: "contain",
          }}
        />
      </div>
      <div className="player-controls">
        <button className="play-btn" onClick={() => setPlaying(!playing)} title={playing ? t("pause") : t("play")}>
          {playing ? <Pause size={13} /> : <Play size={13} />}
        </button>
        <Button variant="ghost" size="icon-sm" onClick={replay} title={t("restart")}>
          <RotateCcw size={12} />
        </Button>
        <span className="frame-indicator">
          {frames.length === 0 ? "0/0" : `${frameIdx + 1}/${frames.length}`}
        </span>
        <input
          type="range"
          style={{ flex: 1 }}
          min={0}
          max={Math.max(0, frames.length - 1)}
          value={Math.min(frameIdx, frames.length - 1)}
          onChange={(e) => scrub(Number(e.target.value))}
        />
        <div className="slider-group">
          <span>FPS {playFps}</span>
          <input type="range" min={1} max={30} value={playFps} onChange={(e) => setPlayFps(Number(e.target.value))} style={{ width: 80 }} />
        </div>
        <div className="slider-group">
          <span>{t("zoom", { n: zoom.toFixed(1) })}</span>
          <input
            type="range"
            min={0.5}
            max={4}
            step={0.25}
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
            style={{ width: 80 }}
          />
        </div>
      </div>
    </>
  );
}
