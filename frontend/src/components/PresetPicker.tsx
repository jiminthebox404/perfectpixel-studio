import { useMemo, useState } from "react";
import { Check, Search } from "lucide-react";
import { PresetInfo } from "../types";
import { useI18n } from "../i18n";
import { presetLabel, categoryLabel } from "../i18n/catalog";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./ui/dialog";
import { Input } from "./ui/input";

interface IProps {
  presets: PresetInfo[];
  usedNames: Set<string>;
  onAdd: (p: PresetInfo) => void;
  onClose: () => void;
}

// 100개 상황 키워드를 카테고리별로 묶고 검색으로 빠르게 찾는 프리셋 선택기.
export default function PresetPicker({ presets, usedNames, onAdd, onClose }: IProps) {
  const { t, lang } = useI18n();
  const [query, setQuery] = useState("");

  // 검색 필터 (번역 라벨 / 한글 라벨 / 영문 이름 / 카테고리 부분일치)
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return presets;
    return presets.filter(
      (p) =>
        presetLabel(p.name, lang, p.label).toLowerCase().includes(q) ||
        p.label.toLowerCase().includes(q) ||
        p.name.toLowerCase().includes(q) ||
        p.category.toLowerCase().includes(q) ||
        categoryLabel(p.category, lang).toLowerCase().includes(q)
    );
  }, [presets, query, lang]);

  // 카탈로그 등장 순서를 유지하며 카테고리별로 그룹화
  const groups = useMemo(() => {
    const map = new Map<string, PresetInfo[]>();
    for (const p of filtered) {
      const arr = map.get(p.category) ?? [];
      arr.push(p);
      map.set(p.category, arr);
    }
    return Array.from(map.entries());
  }, [filtered]);

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="w-[640px] max-h-[80vh] gap-3">
        <div>
          <DialogTitle>{t("picker_title")}</DialogTitle>
          <DialogDescription>
            {t("picker_desc", { n: presets.length })}
          </DialogDescription>
        </div>

        <div className="relative">
          <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            autoFocus
            className="h-8 pl-8"
            placeholder={t("picker_search_ph")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>

        <div className="overflow-y-auto pr-1" style={{ maxHeight: "52vh" }}>
          {groups.length === 0 && (
            <p className="hint py-6 text-center">{t("no_results")}</p>
          )}
          {groups.map(([category, items]) => (
            <div key={category} className="mb-3">
              <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                {categoryLabel(category, lang)} <span className="font-normal">({items.length})</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {items.map((p) => {
                  const used = usedNames.has(p.name);
                  return (
                    <button
                      key={p.name}
                      className="chip"
                      disabled={used}
                      title={t("preset_tip", { action: p.action, frames: p.frames, fps: p.fps, loop: p.loop ? t("loop_suffix") : "" })}
                      onClick={() => onAdd(p)}
                    >
                      {used ? <Check size={11} /> : "+"} {presetLabel(p.name, lang, p.label)}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
