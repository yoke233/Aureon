import { create } from "zustand";
import type { Pipeline } from "../types/workflow";

const mergePipelines = (current: Pipeline[], incoming: Pipeline[]): Pipeline[] => {
  const next = new Map(current.map((pipeline) => [pipeline.id, pipeline]));
  incoming.forEach((pipeline) => {
    const previous = next.get(pipeline.id);
    next.set(pipeline.id, previous ? { ...previous, ...pipeline } : pipeline);
  });
  return Array.from(next.values());
};

interface PipelinesState {
  pipelinesByProjectId: Record<string, Pipeline[]>;
  selectedPipelineId: string | null;
  loading: boolean;
  error: string | null;
  setPipelines: (projectId: string, pipelines: Pipeline[]) => void;
  upsertPipelines: (projectId: string, pipelines: Pipeline[]) => void;
  removePipeline: (projectId: string, pipelineId: string) => void;
  selectPipeline: (pipelineId: string | null) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  reset: () => void;
}

const initialState = {
  pipelinesByProjectId: {} as Record<string, Pipeline[]>,
  selectedPipelineId: null as string | null,
  loading: false,
  error: null as string | null,
};

export const usePipelinesStore = create<PipelinesState>((set) => ({
  ...initialState,
  setPipelines: (projectId, pipelines) =>
    set((state) => ({
      pipelinesByProjectId: {
        ...state.pipelinesByProjectId,
        [projectId]: pipelines,
      },
    })),
  upsertPipelines: (projectId, pipelines) =>
    set((state) => ({
      pipelinesByProjectId: {
        ...state.pipelinesByProjectId,
        [projectId]: mergePipelines(
          state.pipelinesByProjectId[projectId] ?? [],
          pipelines,
        ),
      },
    })),
  removePipeline: (projectId, pipelineId) =>
    set((state) => ({
      pipelinesByProjectId: {
        ...state.pipelinesByProjectId,
        [projectId]: (state.pipelinesByProjectId[projectId] ?? []).filter(
          (pipeline) => pipeline.id !== pipelineId,
        ),
      },
      selectedPipelineId:
        state.selectedPipelineId === pipelineId ? null : state.selectedPipelineId,
    })),
  selectPipeline: (pipelineId) => set({ selectedPipelineId: pipelineId }),
  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),
  reset: () => set({ ...initialState }),
}));
