import { useMemo, useState } from "react";
import type { TaskStep } from "../types/api";

type IssueFlowTreeProps = {
  projectId: string;
  issueId: string;
  steps: TaskStep[];
};

type FlowNode = {
  id: string;
  label: string;
  meta?: string;
  note?: string;
  timestamp?: string;
  kind: "issue" | "run" | "step";
  children: FlowNode[];
};

const ACTION_LABELS: Record<string, string> = {
  created: "Created",
  submitted_for_review: "Submitted for review",
  review_approved: "Review approved",
  review_rejected: "Review rejected",
  queued: "Queued",
  ready: "Ready",
  execution_started: "Execution started",
  merge_started: "Merge started",
  merge_completed: "Merge completed",
  failed: "Failed",
  abandoned: "Abandoned",
  decompose_started: "Decompose started",
  decomposed: "Decomposed",
  superseded: "Superseded",
  run_created: "Run created",
  run_started: "Run started",
  stage_started: "Stage started",
  stage_completed: "Stage completed",
  stage_failed: "Stage failed",
  run_completed: "Run completed",
};

const ACTION_ICONS: Record<string, string> = {
  created: "??",
  submitted_for_review: "??",
  review_approved: "?",
  review_rejected: "?",
  queued: "?",
  ready: "??",
  execution_started: "?",
  merge_started: "??",
  merge_completed: "??",
  failed: "?",
  abandoned: "??",
  decompose_started: "??",
  decomposed: "??",
  superseded: "?",
  run_created: "??",
  run_started: "??",
  stage_started: "?",
  stage_completed: "?",
  stage_failed: "?",
  run_completed: "??",
};

const formatAction = (action: string) => ACTION_LABELS[action] ?? action.replace(/_/g, " ");

const formatTime = (value: string) => {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
};

const buildFlow = (steps: TaskStep[]): FlowNode[] => {
  const ordered = [...steps].sort((left, right) => {
    const leftTime = new Date(left.created_at).getTime();
    const rightTime = new Date(right.created_at).getTime();
    if (Number.isNaN(leftTime) && Number.isNaN(rightTime)) {
      return left.id.localeCompare(right.id);
    }
    if (Number.isNaN(leftTime)) {
      return -1;
    }
    if (Number.isNaN(rightTime)) {
      return 1;
    }
    return leftTime - rightTime;
  });

  const roots: FlowNode[] = [];
  let latestIssueNode: FlowNode | null = null;
  const runNodes = new Map<string, FlowNode>();

  ordered.forEach((step) => {
    const baseNode: FlowNode = {
      id: step.id,
      label: `${ACTION_ICONS[step.action] ?? "?"} ${formatAction(step.action)}`,
      meta: [step.agent_id ? `agent: ${step.agent_id}` : "", step.stage_id ? `stage: ${step.stage_id}` : ""]
        .filter(Boolean)
        .join(" ? "),
      note: step.note || (step.ref_id ? `ref: ${step.ref_type || "unknown"}/${step.ref_id}` : ""),
      timestamp: step.created_at,
      kind: "step",
      children: [],
    };

    if (!step.run_id) {
      const issueNode: FlowNode = {
        ...baseNode,
        id: `issue-${step.id}`,
        kind: "issue",
        children: [],
      };
      roots.push(issueNode);
      latestIssueNode = issueNode;
      return;
    }

    let runNode = runNodes.get(step.run_id);
    if (!runNode) {
      runNode = {
        id: `run-${step.run_id}`,
        label: `?? Run ${step.run_id}`,
        meta: "execution trace",
        kind: "run",
        children: [],
      };
      runNodes.set(step.run_id, runNode);
      if (latestIssueNode) {
        latestIssueNode.children.push(runNode);
      } else {
        roots.push(runNode);
      }
    }
    runNode.children.push(baseNode);
  });

  return roots.reverse();
};

function FlowBranch({ node, level }: { node: FlowNode; level: number }) {
  const [expanded, setExpanded] = useState(true);
  const hasChildren = node.children.length > 0;

  return (
    <li>
      <div
        className="flex items-start gap-2 rounded-md px-2 py-1 hover:bg-[#f6f8fa]"
        style={{ marginLeft: `${level * 16}px` }}
      >
        <button
          type="button"
          className="mt-0.5 h-5 w-5 shrink-0 rounded border border-[#d0d7de] bg-white text-[10px] text-[#57606a] disabled:opacity-40"
          disabled={!hasChildren}
          onClick={() => setExpanded((current) => !current)}
        >
          {hasChildren ? (expanded ? "?" : "+") : "?"}
        </button>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-xs text-[#57606a]">
            <span className="font-semibold text-[#24292f]">{node.label}</span>
            {node.meta ? <span>{node.meta}</span> : null}
            {node.timestamp ? <span className="ml-auto">{formatTime(node.timestamp)}</span> : null}
          </div>
          {node.note ? <p className="mt-1 text-xs text-[#57606a]">{node.note}</p> : null}
        </div>
      </div>
      {expanded && hasChildren ? (
        <ol className="mt-1 space-y-1">
          {node.children.map((child) => (
            <FlowBranch key={child.id} node={child} level={level + 1} />
          ))}
        </ol>
      ) : null}
    </li>
  );
}

export default function IssueFlowTree({ projectId, issueId, steps }: IssueFlowTreeProps) {
  const flow = useMemo(() => buildFlow(steps), [steps]);

  return (
    <section className="rounded-md border border-[#d0d7de] bg-white p-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold text-[#24292f]">Issue Flow</p>
          <p className="text-[11px] text-[#57606a]">
            {projectId} / {issueId}
          </p>
        </div>
        <span className="rounded-full border border-[#d0d7de] px-2 py-0.5 text-[11px] text-[#57606a]">
          {steps.length} steps
        </span>
      </div>

      {flow.length === 0 ? (
        <p className="mt-3 text-xs text-[#57606a]">?? flow ???</p>
      ) : (
        <ol className="mt-3 space-y-1">
          {flow.map((node) => (
            <FlowBranch key={node.id} node={node} level={0} />
          ))}
        </ol>
      )}
    </section>
  );
}
