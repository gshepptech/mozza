import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useDashboard } from "./DashboardPage";
import * as api from "../api/client";
import { Users, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";

export default function CreateTeamPage() {
  const navigate = useNavigate();
  const { refreshTeams, setActiveTeam } = useDashboard();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [loading, setLoading] = useState(false);

  const autoSlug = (val: string) => {
    setName(val);
    setSlug(val.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""));
  };

  const handleCreate = async () => {
    if (!name.trim() || !slug.trim()) return;
    setLoading(true);
    try {
      const team = await api.createTeam(name.trim(), slug.trim());
      toast.success("Team created");
      setActiveTeam(team);
      refreshTeams();
      navigate(`/app/teams/${team.id}`);
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to create team");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto max-w-md pt-12">
      <div className="mb-8 text-center">
        <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand/10 border border-brand/20">
          <Users className="h-7 w-7 text-brand" />
        </div>
        <h1 className="text-2xl font-bold text-foreground">Create a Team</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Teams let you organize recipes and collaborate with your crew.
        </p>
      </div>

      <div className="space-y-5 rounded-xl border border-border bg-card p-6">
        <div className="space-y-2">
          <Label htmlFor="team-name">Team Name</Label>
          <Input
            id="team-name"
            value={name}
            onChange={(e) => autoSlug(e.target.value)}
            placeholder="e.g. Pizza Squad"
            autoFocus
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="team-slug">Slug</Label>
          <Input
            id="team-slug"
            value={slug}
            onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
            placeholder="pizza-squad"
            className="font-mono text-sm"
          />
          <p className="text-xs text-muted-foreground">Used in URLs and API calls</p>
        </div>
        <Button onClick={handleCreate} disabled={loading || !name.trim() || !slug.trim()} className="w-full">
          {loading ? "Creating..." : "Create Team"}
          {!loading && <ArrowRight className="ml-2 h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}
