import type { ReactNode } from "react";

interface ChatPageShellProps {
  sidebar: ReactNode;
  mobileHeader?: ReactNode;
  header?: ReactNode;
  errorBanner?: ReactNode;
  mainPanel: ReactNode;
  permissionBar?: ReactNode;
  inputBar?: ReactNode;
  hiddenFileInput: ReactNode;
}

export function ChatPageShell({
  sidebar,
  mobileHeader,
  header,
  errorBanner,
  mainPanel,
  permissionBar,
  inputBar,
  hiddenFileInput,
}: ChatPageShellProps) {
  return (
    <div className="flex h-full min-w-0 overflow-hidden">
      {sidebar}
      <div className="flex min-w-0 flex-1 flex-col">
        {mobileHeader}
        {header}
        {errorBanner}
        {mainPanel}
        {permissionBar}
        {inputBar}
        {hiddenFileInput}
      </div>
    </div>
  );
}
