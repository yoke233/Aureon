import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  ChevronRight,
  Plus,
  Trash2,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { detectScmProviderFromBinding } from "@/lib/scm";
import { getErrorMessage } from "@/lib/v2Workbench";

interface GitResourceDraftConfig {
  provider: "" | "github" | "codeup";
  enableScmFlow: boolean;
  baseBranch: string;
  mergeMethod: string;
}

interface ResourceDraft {
  kind: string;
  uri: string;
  label: string;
  git: GitResourceDraftConfig;
}

export function CreateProjectPage() {
  const navigate = useNavigate();
  const { apiClient, reloadProjects } = useWorkbench();
  const [name, setName] = useState("");
  const [kind, setKind] = useState<"dev" | "general">("dev");
  const [description, setDescription] = useState("");
  const [resources, setResources] = useState<ResourceDraft[]>([
    {
      kind: "local_fs",
      uri: "",
      label: "工作目录",
      git: { provider: "", enableScmFlow: false, baseBranch: "main", mergeMethod: "squash" },
    },
  ]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const filledResourceCount = useMemo(
    () => resources.filter((resource) => resource.kind.trim() && resource.uri.trim()).length,
    [resources],
  );

  const updateResource = (index: number, patch: Partial<ResourceDraft>) => {
    setResources((current) =>
      current.map((resource, currentIndex) =>
        currentIndex === index ? { ...resource, ...patch } : resource,
      ),
    );
  };

  const updateGitResource = (index: number, patch: Partial<GitResourceDraftConfig>) => {
    setResources((current) =>
      current.map((resource, currentIndex) => {
        if (currentIndex === index) {
          return { ...resource, git: { ...resource.git, ...patch } };
        }
        if (patch.enableScmFlow && resource.kind === "git") {
          return { ...resource, git: { ...resource.git, enableScmFlow: false } };
        }
        return resource;
      }),
    );
  };

  const addResource = () => {
    setResources((current) => [...current, {
      kind: "git",
      uri: "",
      label: "",
      git: { provider: "", enableScmFlow: false, baseBranch: "main", mergeMethod: "squash" },
    }]);
  };

  const removeResource = (index: number) => {
    setResources((current) => current.filter((_, currentIndex) => currentIndex !== index));
  };

  const createProject = async () => {
    if (!name.trim()) {
      setError("项目名称不能为空。");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      const project = await apiClient.createProject({
        name: name.trim(),
        kind,
        description: description.trim() || undefined,
      });

      const nextResources = resources.filter((resource) => resource.kind.trim() && resource.uri.trim());
      for (const resource of nextResources) {
        const scmProvider = resource.kind.trim() === "git"
          ? detectScmProviderFromBinding({
              kind: resource.kind,
              uri: resource.uri,
              config: resource.git.provider ? { provider: resource.git.provider } : {},
            })
          : null;
        const config = resource.kind.trim() === "git"
          ? {
              ...(scmProvider ? { provider: scmProvider } : {}),
              ...(scmProvider ? { enable_scm_flow: resource.git.enableScmFlow } : {}),
              ...(scmProvider && resource.git.baseBranch.trim() ? { base_branch: resource.git.baseBranch.trim() } : {}),
              ...(scmProvider && resource.git.mergeMethod.trim() ? { merge_method: resource.git.mergeMethod.trim() } : {}),
            }
          : undefined;
        await apiClient.createProjectResource(project.id, {
          kind: resource.kind.trim(),
          uri: resource.uri.trim(),
          config: config && Object.keys(config).length > 0 ? config : undefined,
          label: resource.label.trim() || undefined,
        });
      }

      await reloadProjects(project.id);
      navigate("/projects");
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex-1 space-y-6 p-8">
      <div>
        <div className="mb-1 flex items-center gap-2 text-sm text-muted-foreground">
          <Link to="/projects" className="hover:text-foreground">项目</Link>
          <ChevronRight className="h-3 w-3" />
          <span className="font-medium text-foreground">新建项目</span>
        </div>
        <h1 className="text-2xl font-bold tracking-tight">新建项目</h1>
        <p className="text-sm text-muted-foreground">真实写入项目表，并可同时创建资源绑定。</p>
      </div>

      {error ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

      <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">基本信息</CardTitle>
              <CardDescription>填写项目的基本配置信息</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="space-y-1.5">
                <label className="text-sm font-medium">项目名称</label>
                <Input
                  placeholder="例如：ai-workflow"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-sm font-medium">项目类型</label>
                <select
                  className="flex h-10 w-full rounded-md border bg-background px-3 text-sm"
                  value={kind}
                  onChange={(event) => setKind(event.target.value as "dev" | "general")}
                >
                  <option value="dev">dev</option>
                  <option value="general">general</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <label className="text-sm font-medium">项目描述</label>
                <textarea
                  className="flex min-h-[80px] w-full rounded-md border bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                  placeholder="描述项目的目标、技术栈和范围..."
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                />
              </div>

            </CardContent>
          </Card>

          <Card className="overflow-hidden p-0">
            <div className="flex items-center justify-between border-b px-6 py-5">
              <div>
                <h3 className="text-base font-semibold">资源绑定</h3>
                <p className="mt-1 text-[13px] text-muted-foreground">支持本地目录、git 仓库等真实资源。</p>
              </div>
              <Button variant="outline" size="sm" className="gap-1.5" onClick={addResource}>
                <Plus className="h-3.5 w-3.5" />
                添加资源
              </Button>
            </div>
            <div className="space-y-4 p-6">
              {resources.map((resource, index) => (
                <div key={`${resource.kind}-${index}`} className="rounded-lg border p-4">
                  <div className="grid gap-3 md:grid-cols-[120px_minmax(0,1fr)_120px_40px]">
                    <select
                      className="h-10 rounded-md border bg-background px-3 text-sm"
                      value={resource.kind}
                      onChange={(event) => updateResource(index, { kind: event.target.value })}
                    >
                      <option value="local_fs">local_fs</option>
                      <option value="git">git</option>
                      <option value="s3">s3</option>
                    </select>
                    <Input
                      placeholder="URI / 路径 / 仓库地址"
                      value={resource.uri}
                      onChange={(event) => {
                        const uri = event.target.value;
                        const detected = detectScmProviderFromBinding({
                          kind: resource.kind,
                          uri,
                          config: {},
                        });
                        updateResource(index, { uri });
                        if (resource.kind === "git" && detected) {
                          updateGitResource(index, { provider: detected });
                        }
                      }}
                    />
                    <Input
                      placeholder="标签"
                      value={resource.label}
                      onChange={(event) => updateResource(index, { label: event.target.value })}
                    />
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-10 w-10"
                      aria-label={`删除第 ${index + 1} 个资源`}
                      onClick={() => removeResource(index)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                  {resource.kind === "git" ? (
                    (() => {
                      const scmProvider = detectScmProviderFromBinding({
                        kind: "git",
                        uri: resource.uri,
                        config: resource.git.provider ? { provider: resource.git.provider } : {},
                      });
                      return (
                        <div className="mt-3 space-y-3">
                          <div className="grid gap-3 md:grid-cols-3">
                            <div className="space-y-1.5">
                              <label className="text-xs font-medium text-muted-foreground">Provider</label>
                              <select
                                className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                                value={resource.git.provider}
                                onChange={(event) => updateGitResource(index, {
                                  provider: event.target.value as GitResourceDraftConfig["provider"],
                                })}
                              >
                                <option value="">自动识别/未指定</option>
                                <option value="github">github</option>
                                <option value="codeup">codeup</option>
                              </select>
                            </div>
                            {scmProvider ? (
                              <>
                                <label className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm md:col-span-1">
                                  <input
                                    type="checkbox"
                                    checked={resource.git.enableScmFlow}
                                    onChange={(event) => updateGitResource(index, { enableScmFlow: event.target.checked })}
                                  />
                                  <span>启用 PR/CR 流程</span>
                                </label>
                                <div className="flex items-center text-xs text-muted-foreground">
                                  当前识别：{scmProvider === "codeup" ? "Codeup CR" : "GitHub PR"}
                                </div>
                              </>
                            ) : (
                              <div className="md:col-span-2 flex items-center text-xs text-muted-foreground">
                                仅 GitHub / Codeup 仓库支持 PR/CR 流程开关。
                              </div>
                            )}
                          </div>
                          {scmProvider ? (
                            <div className="space-y-2">
                              <div className="text-[11px] text-muted-foreground">
                                同一个项目当前只建议启用一个 GitHub / Codeup PR/CR 资源。
                              </div>
                              <div className="grid gap-3 md:grid-cols-2">
                                <div className="space-y-1.5">
                                  <label className="text-xs font-medium text-muted-foreground">Base Branch</label>
                                  <Input
                                    placeholder="main / master"
                                    value={resource.git.baseBranch}
                                    onChange={(event) => updateGitResource(index, { baseBranch: event.target.value })}
                                  />
                                </div>
                                <div className="space-y-1.5">
                                  <label className="text-xs font-medium text-muted-foreground">Merge Method</label>
                                  <select
                                    className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                                    value={resource.git.mergeMethod}
                                    onChange={(event) => updateGitResource(index, { mergeMethod: event.target.value })}
                                  >
                                    <option value="squash">squash</option>
                                    <option value="merge">merge</option>
                                    <option value="rebase">rebase</option>
                                  </select>
                                </div>
                              </div>
                            </div>
                          ) : null}
                        </div>
                      );
                    })()
                  ) : null}
                </div>
              ))}
            </div>
          </Card>
        </div>

        <div className="space-y-6">
          <Card className="overflow-hidden p-0">
            <div className="border-b px-5 py-3.5">
              <span className="text-sm font-semibold">项目预览</span>
            </div>
            <div className="space-y-4 p-5">
              <div className="flex justify-between text-[13px]">
                <span className="text-muted-foreground">名称</span>
                <span className="font-medium">{name || "未填写"}</span>
              </div>
              <div className="flex items-center justify-between text-[13px]">
                <span className="text-muted-foreground">类型</span>
                <Badge variant="secondary">{kind}</Badge>
              </div>
              <div className="flex justify-between text-[13px]">
                <span className="text-muted-foreground">有效资源</span>
                <span className="font-medium">{filledResourceCount} 个绑定</span>
              </div>
              <div className="flex justify-between gap-3 text-[13px]">
                <span className="text-muted-foreground">描述</span>
                <span className="text-right font-medium">{description || "未填写"}</span>
              </div>
            </div>
          </Card>

          <div className="space-y-2.5">
            <Button className="w-full gap-2" disabled={submitting} onClick={() => void createProject()}>
              <Plus className="h-4 w-4" />
              {submitting ? "创建中..." : "创建项目"}
            </Button>
            <Link to="/projects" className="block">
              <Button variant="ghost" className="w-full text-muted-foreground">
                取消
              </Button>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}


