import { create } from "zustand";
import type { TaskItem, TaskPlan, TaskPlanStatus } from "../types/workflow";

const upsertPlan = (plans: TaskPlan[], incoming: TaskPlan): TaskPlan[] => {
  const index = plans.findIndex((plan) => plan.id === incoming.id);
  if (index < 0) {
    return [...plans, incoming];
  }
  const next = plans.slice();
  next[index] = { ...next[index], ...incoming };
  return next;
};

interface PlansState {
  plansByProjectId: Record<string, TaskPlan[]>;
  activePlanId: string | null;
  loading: boolean;
  error: string | null;
  setPlans: (projectId: string, plans: TaskPlan[]) => void;
  upsertPlan: (projectId: string, plan: TaskPlan) => void;
  setPlanTasks: (projectId: string, planId: string, tasks: TaskItem[]) => void;
  updatePlanStatus: (
    projectId: string,
    planId: string,
    status: TaskPlanStatus,
  ) => void;
  selectPlan: (planId: string | null) => void;
  removePlan: (projectId: string, planId: string) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  reset: () => void;
}

const initialState = {
  plansByProjectId: {} as Record<string, TaskPlan[]>,
  activePlanId: null as string | null,
  loading: false,
  error: null as string | null,
};

export const usePlansStore = create<PlansState>((set) => ({
  ...initialState,
  setPlans: (projectId, plans) =>
    set((state) => ({
      plansByProjectId: {
        ...state.plansByProjectId,
        [projectId]: plans,
      },
    })),
  upsertPlan: (projectId, plan) =>
    set((state) => ({
      plansByProjectId: {
        ...state.plansByProjectId,
        [projectId]: upsertPlan(state.plansByProjectId[projectId] ?? [], plan),
      },
    })),
  setPlanTasks: (projectId, planId, tasks) =>
    set((state) => ({
      plansByProjectId: {
        ...state.plansByProjectId,
        [projectId]: (state.plansByProjectId[projectId] ?? []).map((plan) =>
          plan.id === planId ? { ...plan, tasks } : plan,
        ),
      },
    })),
  updatePlanStatus: (projectId, planId, status) =>
    set((state) => ({
      plansByProjectId: {
        ...state.plansByProjectId,
        [projectId]: (state.plansByProjectId[projectId] ?? []).map((plan) =>
          plan.id === planId ? { ...plan, status } : plan,
        ),
      },
    })),
  selectPlan: (planId) => set({ activePlanId: planId }),
  removePlan: (projectId, planId) =>
    set((state) => ({
      plansByProjectId: {
        ...state.plansByProjectId,
        [projectId]: (state.plansByProjectId[projectId] ?? []).filter(
          (plan) => plan.id !== planId,
        ),
      },
      activePlanId: state.activePlanId === planId ? null : state.activePlanId,
    })),
  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),
  reset: () => set({ ...initialState }),
}));
