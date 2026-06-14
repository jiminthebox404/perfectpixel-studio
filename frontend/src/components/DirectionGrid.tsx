import { useEffect, useState } from "react";
import { Compass, FlipHorizontal2, Loader2 } from "lucide-react";
import { DirectionInfo, StateDef, selectedFrames } from "../types";
import { useI18n } from "../i18n";
import { directionName } from "../i18n/catalog";

interface IProps {
  states: StateDef[]; // 같은 dirBase에 속한 방향 상태들
  directions: DirectionInfo[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

// 8방향 세트 3x3 미리보기 그리드
export default function DirectionGrid({ states, directions, selectedId, onSelect }: IProps) {
  const { t, lang } = useI18n();
  const cells: (DirectionInfo | null)[] = Array(9).fill(null);
  directions.forEach((d) => {
    cells[d.row * 3 + d.col] = d;
  });

  return (
    <div className="dir-grid checker">
      {cells.map((d, i) => {
        if (i === 4) {
          return (
            <div key="center" className="dir-cell dir-center">
              <Compass size={16} />
              <span>{t("dir8")}</span>
            </div>
          );
        }
        if (!d) return <div key={i} className="dir-cell dir-center" />;
        const st = states.find((s) => s.facing === d.key);
        const frames = st ? selectedFrames(st).map((f) => f.png) : [];
        return (
          <div
            key={d.key}
            className={`dir-cell ${st && st.id === selectedId ? "active" : ""} ${st ? "clickable" : ""}`}
            title={`${directionName(d.key, lang, d.label)} (${d.short})${d.mirrorOf ? t("dir_mirror_suffix") : ""}`}
            onClick={() => st && onSelect(st.id)}
          >
            {st?.status === "generating" ? (
              <Loader2 size={14} className="animate-spin" />
            ) : frames.length > 0 ? (
              <MiniAnim frames={frames} fps={st!.fps} />
            ) : (
              <span className={`dir-dot ${st?.status ?? "none"}`} />
            )}
            <span className="dir-label">
              {d.mirrorOf && <FlipHorizontal2 size={8} style={{ display: "inline" }} />} {d.short}
            </span>
          </div>
        );
      })}
    </div>
  );
}

// 작은 자동 재생 프리뷰 (컨트롤 없음)
function MiniAnim({ frames, fps }: { frames: string[]; fps: number }) {
  const [idx, setIdx] = useState(0);
  // frames 배열은 렌더마다 새로 생성되므로 길이/fps 변화에만 인터벌을 재설정 (리렌더 시 애니메이션 리셋 방지)
  useEffect(() => {
    if (frames.length <= 1) return;
    const t = setInterval(() => setIdx((i) => (i + 1) % frames.length), 1000 / Math.max(1, Math.min(30, fps)));
    return () => clearInterval(t);
  }, [frames.length, fps]);
  return <img src={frames[idx % frames.length]} className="pixelated" alt="" />;
}
