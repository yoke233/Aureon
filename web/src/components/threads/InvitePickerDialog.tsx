import { useTranslation } from "react-i18next";
import { Bot, Check, Loader2, Plus, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { AgentProfile } from "@/types/apiV2";

interface InvitePickerDialogProps {
  candidates: AgentProfile[];
  selectedIDs: Set<string>;
  busy: boolean;
  onToggle: (profileID: string) => void;
  onClose: () => void;
  onConfirm: () => void;
}

export function InvitePickerDialog({
  candidates,
  selectedIDs,
  busy,
  onToggle,
  onClose,
  onConfirm,
}: InvitePickerDialogProps) {
  const { t } = useTranslation();

  if (candidates.length === 0) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="mx-4 w-full max-w-md rounded-2xl border bg-background shadow-2xl">
        <div className="flex items-center justify-between border-b px-5 py-3.5">
          <div>
            <h3 className="text-sm font-semibold">{t("threads.invitePickerTitle", "Select agents to invite")}</h3>
            <p className="mt-0.5 text-[11px] text-muted-foreground">
              {t("threads.invitePickerHint", "Multiple agents matched. Select the ones you want to invite.")}
            </p>
          </div>
          <button
            type="button"
            className="flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={onClose}
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="max-h-72 overflow-y-auto p-3">
          <div className="space-y-1.5">
            {candidates.map((profile) => {
              const isSelected = selectedIDs.has(profile.id);
              return (
                <button
                  key={profile.id}
                  type="button"
                  className={cn(
                    "flex w-full items-center gap-3 rounded-xl border p-3 text-left transition-all",
                    isSelected
                      ? "border-blue-300 bg-blue-50 shadow-sm"
                      : "border-border/60 hover:border-border hover:bg-muted/30",
                    busy && "pointer-events-none opacity-60",
                  )}
                  onClick={() => onToggle(profile.id)}
                  disabled={busy}
                >
                  <div className={cn(
                    "flex h-5 w-5 shrink-0 items-center justify-center rounded border transition-colors",
                    isSelected ? "border-blue-500 bg-blue-500 text-white" : "border-slate-300 bg-white",
                  )}>
                    {isSelected && <Check className="h-3 w-3" />}
                  </div>
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
                    <Bot className="h-4 w-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate text-sm font-medium">{profile.name ?? profile.id}</span>
                      <Badge variant="outline" className="shrink-0 text-[9px]">{profile.role}</Badge>
                    </div>
                    {profile.name && (
                      <p className="mt-0.5 truncate text-[11px] text-muted-foreground">@{profile.id}</p>
                    )}
                    <p className="mt-0.5 truncate text-[10px] text-muted-foreground">
                      {profile.driver_id ?? profile.driver?.launch_command ?? "-"}
                      {profile.capabilities && profile.capabilities.length > 0 && (
                        <> | {profile.capabilities.slice(0, 3).join(", ")}{profile.capabilities.length > 3 ? "..." : ""}</>
                      )}
                    </p>
                  </div>
                </button>
              );
            })}
          </div>
        </div>
        <div className="flex items-center justify-between border-t px-5 py-3">
          <span className="text-xs text-muted-foreground">
            {selectedIDs.size > 0
              ? t("threads.invitePickerCount", { defaultValue: "{{count}} selected", count: selectedIDs.size })
              : t("threads.invitePickerNone", "None selected")}
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
              {t("common.cancel", "Cancel")}
            </Button>
            <Button size="sm" onClick={onConfirm} disabled={selectedIDs.size === 0 || busy}>
              {busy ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Plus className="mr-1.5 h-3.5 w-3.5" />
              )}
              {t("threads.invitePickerConfirm", "Invite")} {selectedIDs.size > 0 ? `(${selectedIDs.size})` : ""}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
