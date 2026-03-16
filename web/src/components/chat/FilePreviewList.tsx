import { useEffect, useMemo, useState } from "react";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";

function ImagePreviewBadge({ file, label, onRemove }: { file: File; label: string; onRemove: () => void }) {
  const [hover, setHover] = useState(false);
  const previewUrl = useMemo(() => URL.createObjectURL(file), [file]);
  useEffect(() => () => URL.revokeObjectURL(previewUrl), [previewUrl]);

  return (
    <div
      className="relative"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <Badge variant="secondary" className="gap-1 text-xs">
        {label}
        <button type="button" onClick={onRemove} className="ml-1 hover:text-red-500">
          <X className="h-3 w-3" />
        </button>
      </Badge>
      {hover && (
        <div className="absolute bottom-full left-0 z-50 mb-2 rounded-lg border bg-popover p-1.5 shadow-lg">
          <img
            src={previewUrl}
            alt={file.name}
            className="max-h-48 max-w-64 rounded object-contain"
          />
          <div className="mt-1 text-center text-[10px] text-muted-foreground">{file.name}</div>
        </div>
      )}
    </div>
  );
}

export function FilePreviewList({
  files,
  onRemove,
}: {
  files: File[];
  onRemove: (index: number) => void;
}) {
  if (files.length === 0) return null;

  const showIndex = files.length > 1;

  return (
    <div className="flex flex-wrap gap-2">
      {files.map((file, idx) => {
        const label = showIndex ? `${idx + 1}. ${file.name}` : file.name;
        return file.type.startsWith("image/") ? (
          <ImagePreviewBadge key={idx} file={file} label={label} onRemove={() => onRemove(idx)} />
        ) : (
          <Badge key={idx} variant="secondary" className="gap-1 text-xs">
            {label}
            <button type="button" onClick={() => onRemove(idx)} className="ml-1 hover:text-red-500">
              <X className="h-3 w-3" />
            </button>
          </Badge>
        );
      })}
    </div>
  );
}
