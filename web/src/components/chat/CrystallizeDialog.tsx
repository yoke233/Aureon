import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";

interface CrystallizeDialogProps {
  open: boolean;
  crystallizing: boolean;
  threadTitle: string;
  threadSummary: string;
  workItemTitle: string;
  workItemBody: string;
  createWorkItem: boolean;
  onClose: () => void;
  onThreadTitleChange: (value: string) => void;
  onThreadSummaryChange: (value: string) => void;
  onWorkItemTitleChange: (value: string) => void;
  onWorkItemBodyChange: (value: string) => void;
  onCreateWorkItemChange: (checked: boolean) => void;
  onSubmit: () => void;
}

export function CrystallizeDialog({
  open,
  crystallizing,
  threadTitle,
  threadSummary,
  workItemTitle,
  workItemBody,
  createWorkItem,
  onClose,
  onThreadTitleChange,
  onThreadSummaryChange,
  onWorkItemTitleChange,
  onWorkItemBodyChange,
  onCreateWorkItemChange,
  onSubmit,
}: CrystallizeDialogProps) {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onClose={() => !crystallizing && onClose()}>
      <DialogHeader>
        <DialogTitle>{t("chat.crystallizeDialogTitle", { defaultValue: "结晶为 Thread / Work Item" })}</DialogTitle>
        <DialogDescription>
          {t("chat.crystallizeDialogDesc", {
            defaultValue: "把当前 chat session 固化为 thread，并按需同步创建 work item。",
          })}
        </DialogDescription>
      </DialogHeader>
      <DialogBody>
        <div className="space-y-2">
          <label htmlFor="crystallize-thread-title" className="text-sm font-medium text-slate-900">
            {t("chat.crystallizeThreadTitle", { defaultValue: "Thread 标题" })}
          </label>
          <Input
            id="crystallize-thread-title"
            value={threadTitle}
            onChange={(e) => onThreadTitleChange(e.target.value)}
            placeholder={t("chat.crystallizeThreadTitlePlaceholder", { defaultValue: "输入 thread 标题" })}
            disabled={crystallizing}
          />
        </div>
        <div className="space-y-2">
          <label htmlFor="crystallize-thread-summary" className="text-sm font-medium text-slate-900">
            {t("chat.crystallizeThreadSummary", { defaultValue: "Thread 摘要" })}
          </label>
          <Textarea
            id="crystallize-thread-summary"
            value={threadSummary}
            onChange={(e) => onThreadSummaryChange(e.target.value)}
            placeholder={t("chat.crystallizeThreadSummaryPlaceholder", { defaultValue: "输入 thread 摘要" })}
            disabled={crystallizing}
            className="min-h-[140px]"
          />
        </div>
        <label className="flex items-center gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={createWorkItem}
            onChange={(e) => onCreateWorkItemChange(e.target.checked)}
            disabled={crystallizing}
            className="h-4 w-4 rounded border-slate-300"
          />
          <span>{t("chat.crystallizeCreateWorkItem", { defaultValue: "同时创建 work item" })}</span>
        </label>
        {createWorkItem ? (
          <>
            <div className="space-y-2">
              <label htmlFor="crystallize-work-item-title" className="text-sm font-medium text-slate-900">
                {t("chat.crystallizeWorkItemTitle", { defaultValue: "Work item 标题" })}
              </label>
              <Input
                id="crystallize-work-item-title"
                value={workItemTitle}
                onChange={(e) => onWorkItemTitleChange(e.target.value)}
                placeholder={t("chat.crystallizeWorkItemTitlePlaceholder", { defaultValue: "输入 work item 标题" })}
                disabled={crystallizing}
              />
            </div>
            <div className="space-y-2">
              <label htmlFor="crystallize-work-item-body" className="text-sm font-medium text-slate-900">
                {t("chat.crystallizeWorkItemBody", { defaultValue: "Work item 内容" })}
              </label>
              <Textarea
                id="crystallize-work-item-body"
                value={workItemBody}
                onChange={(e) => onWorkItemBodyChange(e.target.value)}
                placeholder={t("chat.crystallizeWorkItemBodyPlaceholder", {
                  defaultValue: "可留空，后端会回退到 thread summary。",
                })}
                disabled={crystallizing}
                className="min-h-[120px]"
              />
            </div>
          </>
        ) : null}
      </DialogBody>
      <DialogFooter>
        <button
          type="button"
          className="h-10 rounded-md border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
          onClick={onClose}
          disabled={crystallizing}
        >
          {t("common.cancel", { defaultValue: "取消" })}
        </button>
        <button
          type="button"
          className="inline-flex h-10 items-center justify-center rounded-md bg-slate-900 px-4 text-sm font-medium text-white transition-colors hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          onClick={onSubmit}
          disabled={crystallizing}
        >
          {crystallizing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t("chat.crystallizing", { defaultValue: "结晶中..." })}
            </>
          ) : (
            t("chat.crystallizeConfirm", { defaultValue: "确认结晶" })
          )}
        </button>
      </DialogFooter>
    </Dialog>
  );
}
