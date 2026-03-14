// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import type React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { AttachmentUploader } from "./AttachmentUploader";

function renderUploader(props?: Partial<React.ComponentProps<typeof AttachmentUploader>>) {
  const onFilesSelected = vi.fn();
  const onRemovePending = vi.fn();
  const onRemoveUploaded = vi.fn();

  const result = render(
    <I18nextProvider i18n={i18n}>
      <AttachmentUploader
        pendingFiles={[new File(["hello"], "draft.md", { type: "text/markdown" })]}
        uploadedAttachments={[
          { id: 7, file_name: "diagram.png", mime_type: "image/png" },
        ]}
        uploading
        onFilesSelected={onFilesSelected}
        onRemovePending={onRemovePending}
        onRemoveUploaded={onRemoveUploaded}
        {...props}
      />
    </I18nextProvider>,
  );

  return { ...result, onFilesSelected, onRemovePending, onRemoveUploaded };
}

describe("AttachmentUploader", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("展示待上传与已上传附件，并支持移除", () => {
    const { onRemovePending, onRemoveUploaded } = renderUploader();

    expect(screen.getByText("draft.md")).toBeTruthy();
    expect(screen.getByText("diagram.png")).toBeTruthy();
    expect(screen.getByText("上传中...")).toBeTruthy();

    fireEvent.click(within(screen.getByText("draft.md").parentElement as HTMLElement).getByRole("button"));
    expect(onRemovePending).toHaveBeenCalledWith(0);

    fireEvent.click(within(screen.getByText("diagram.png").parentElement as HTMLElement).getByRole("button"));
    expect(onRemoveUploaded).toHaveBeenCalledWith({
      id: 7,
      file_name: "diagram.png",
      mime_type: "image/png",
    });
  });

  it("支持拖拽和选择文件", () => {
    const { onFilesSelected } = renderUploader({
      pendingFiles: [],
      uploadedAttachments: [],
      uploading: false,
    });

    const dropzone = screen.getByText("拖拽或点击上传 .md、.txt 或图片文件").closest("div")?.parentElement as HTMLElement;
    fireEvent.dragOver(dropzone, {
      dataTransfer: { files: [] },
    });
    expect(screen.getByText("拖拽文件到此处上传")).toBeTruthy();

    const dragFile = new File(["drop"], "drop.txt", { type: "text/plain" });
    fireEvent.drop(dropzone, {
      dataTransfer: { files: [dragFile] },
    });
    expect(onFilesSelected).toHaveBeenCalledWith([dragFile]);

    const browseFile = new File(["pick"], "pick.md", { type: "text/markdown" });
    const input = document.querySelector("input[type='file']") as HTMLInputElement;
    fireEvent.change(input, { target: { files: [browseFile] } });

    expect(onFilesSelected).toHaveBeenLastCalledWith([browseFile]);
  });
});
