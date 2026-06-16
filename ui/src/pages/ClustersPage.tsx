import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useDashboard } from "./DashboardPage";
import * as api from "../api/client";
import { Server, Plus, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/custom/empty-state";
import { toast } from "sonner";

interface Cluster {
  id: string;
  name: string;
  created_at: string;
}

export default function ClustersPage() {
  const { activeTeam } = useDashboard();
  const navigate = useNavigate();
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [kubeconfig, setKubeconfig] = useState("");
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);

  const fetchClusters = () => {
    setLoading(true);
    api.listClusters()
      .then(({ clusters }) => setClusters(clusters ?? []))
      .catch(() => toast.error("Failed to load clusters"))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchClusters();
  }, []);

  const handleCreate = async () => {
    if (!name.trim() || !kubeconfig.trim()) return;
    setCreating(true);
    try {
      await api.createCluster(name.trim(), kubeconfig.trim());
      toast.success("Cluster registered");
      setName("");
      setKubeconfig("");
      setShowForm(false);
      fetchClusters();
    } catch {
      toast.error("Failed to register cluster");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteCluster(id);
      setClusters(clusters.filter(c => c.id !== id));
      setConfirmDeleteId(null);
      toast.success("Cluster removed");
    } catch {
      toast.error("Failed to remove cluster");
    }
  };

  if (!activeTeam) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <EmptyState
          icon={Server}
          title="No team selected"
          description="Create a team to manage clusters"
          action={{ label: "Create Team", onClick: () => navigate("/app/teams/new") }}
        />
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
      {/* Header */}
      <div className="flex items-start justify-between mb-10">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">Clusters</h1>
          <p className="text-sm text-muted-foreground mt-1.5">
            Manage registered Kubernetes clusters
          </p>
        </div>
        <Button
          onClick={() => setShowForm(true)}
          className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.15)]"
        >
          <Plus className="mr-2 h-4 w-4" />
          Add Cluster
        </Button>
      </div>

      {/* Divider */}
      <div className="pizza-cutter mb-10" />

      {/* Add Cluster form */}
      {showForm && (
        <Card className="oven-card bg-card border-brand/20 mb-8">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 p-6 pb-4">
            <CardTitle className="text-sm font-bold">Register a new cluster</CardTitle>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground"
              onClick={() => { setShowForm(false); setName(""); setKubeconfig(""); }}
            >
              <X className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent className="px-6 pb-6 space-y-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground font-medium">Cluster name</Label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. production-us-east"
                className="h-10 text-sm bg-elevated"
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground font-medium">Kubeconfig</Label>
              <textarea
                value={kubeconfig}
                onChange={(e) => setKubeconfig(e.target.value)}
                placeholder="Paste your kubeconfig YAML here..."
                rows={8}
                className="flex w-full rounded-md border border-input bg-elevated px-3 py-2 text-sm font-mono ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 resize-y"
              />
            </div>
            <div className="flex justify-end gap-3 pt-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => { setShowForm(false); setName(""); setKubeconfig(""); }}
              >
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={!name.trim() || !kubeconfig.trim() || creating}
                onClick={handleCreate}
                className="bg-brand hover:bg-brand-hover text-primary-foreground"
              >
                {creating ? "Registering..." : "Register Cluster"}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Cluster list */}
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map(i => (
            <Card key={i} className="oven-card bg-card border-border/50">
              <CardHeader className="p-6 pb-3"><Skeleton className="h-5 w-32" /></CardHeader>
              <CardContent className="px-6 pb-6"><Skeleton className="h-10 w-full rounded-lg" /></CardContent>
            </Card>
          ))}
        </div>
      ) : clusters.length === 0 && !showForm ? (
        <EmptyState
          icon={Server}
          title="No clusters registered"
          description="Add a cluster to start deploying to Kubernetes"
          action={{ label: "Add Cluster", onClick: () => setShowForm(true) }}
        />
      ) : clusters.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {clusters.map(cluster => (
            <Card key={cluster.id} className="oven-card feature-card bg-card border-border/50">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 p-6 pb-3">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-brand/10 border border-brand/15">
                    <Server className="h-4 w-4 text-brand" />
                  </div>
                  <CardTitle className="text-sm font-bold">{cluster.name}</CardTitle>
                </div>
              </CardHeader>
              <CardContent className="px-6 pb-4">
                <span className="text-xs text-muted-foreground font-mono">
                  Created {new Date(cluster.created_at).toLocaleDateString()}
                </span>
              </CardContent>
              <CardFooter className="border-t border-border/40 px-6 py-4 flex items-center justify-between">
                <span className="text-xs text-muted-foreground font-mono">{cluster.id.slice(0, 8)}</span>
                {confirmDeleteId === cluster.id ? (
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-destructive">Confirm?</span>
                    <Button
                      variant="destructive"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => handleDelete(cluster.id)}
                    >
                      Delete
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => setConfirmDeleteId(null)}
                    >
                      Cancel
                    </Button>
                  </div>
                ) : (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-muted-foreground hover:text-destructive"
                    onClick={() => setConfirmDeleteId(cluster.id)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                )}
              </CardFooter>
            </Card>
          ))}
        </div>
      ) : null}
    </div>
  );
}
