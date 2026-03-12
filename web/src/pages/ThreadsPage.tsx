import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Plus, Search, MessagesSquare, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import type { Thread } from "@/types/apiV2";

export function ThreadsPage() {
  const { t } = useTranslation();
  const { apiClient } = useWorkbench();
  const [search, setSearch] = useState("");
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState("");

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const listed = await apiClient.listThreads({ limit: 200 });
        if (!cancelled) setThreads(listed);
      } catch (e) {
        if (!cancelled) setError(getErrorMessage(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => { cancelled = true; };
  }, [apiClient]);

  const filtered = useMemo(() => {
    if (!search.trim()) return threads;
    const q = search.toLowerCase();
    return threads.filter(
      (th) =>
        th.title.toLowerCase().includes(q) ||
        String(th.id).includes(q),
    );
  }, [threads, search]);

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    try {
      const created = await apiClient.createThread({ title: newTitle.trim() });
      setThreads((prev) => [created, ...prev]);
      setNewTitle("");
      setShowCreate(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  const statusVariant = (status: string) => {
    switch (status) {
      case "active": return "default";
      case "closed": return "secondary";
      case "archived": return "outline";
      default: return "default";
    }
  };

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">
          {t("nav.threads")}
        </h1>
        <Button size="sm" onClick={() => setShowCreate(!showCreate)}>
          <Plus className="mr-1.5 h-4 w-4" />
          {t("threads.create", "New Thread")}
        </Button>
      </div>

      {showCreate && (
        <Card>
          <CardContent className="pt-4">
            <div className="flex gap-2">
              <Input
                placeholder={t("threads.titlePlaceholder", "Thread title...")}
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleCreate()}
              />
              <Button onClick={handleCreate} disabled={!newTitle.trim()}>
                {t("threads.createBtn", "Create")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder={t("threads.search", "Search threads...")}
          className="pl-9"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <MessagesSquare className="h-4 w-4" />
            {t("nav.threads")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : filtered.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              {t("threads.empty", "No threads yet")}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-16">ID</TableHead>
                  <TableHead>{t("threads.title", "Title")}</TableHead>
                  <TableHead className="w-24">{t("threads.status", "Status")}</TableHead>
                  <TableHead className="w-32">{t("threads.owner", "Owner")}</TableHead>
                  <TableHead className="w-40">{t("threads.updated", "Updated")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((th) => (
                  <TableRow key={th.id}>
                    <TableCell className="font-mono text-xs">{th.id}</TableCell>
                    <TableCell>
                      <Link
                        to={`/threads/${th.id}`}
                        className="font-medium text-primary hover:underline"
                      >
                        {th.title}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(th.status)}>{th.status}</Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {th.owner_id || "—"}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatRelativeTime(th.updated_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
