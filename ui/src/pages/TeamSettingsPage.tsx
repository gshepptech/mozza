import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useDashboard } from "./DashboardPage";
import * as api from "../api/client";
import type { TeamMember } from "../api/types";
import { Users, UserPlus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { EmptyState } from "@/components/custom/empty-state";
import { toast } from "sonner";

export default function TeamSettingsPage() {
  const { activeTeam, refreshTeams } = useDashboard();
  const navigate = useNavigate();
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [email, setEmail] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [membersError, setMembersError] = useState<string | null>(null);

  useEffect(() => {
    if (!activeTeam) return;
    setMembersError(null);
    api.listTeamMembers(activeTeam.id)
      .then(({ members }) => setMembers(members))
      .catch((err) => {
        setMembersError(err instanceof Error ? err.message : "Failed to load members");
      });
  }, [activeTeam?.id]);

  const handleAdd = async () => {
    if (!activeTeam || !email) return;
    try {
      await api.addTeamMember(activeTeam.id, email, "member");
      setEmail("");
      setAddOpen(false);
      const { members } = await api.listTeamMembers(activeTeam.id);
      setMembers(members);
      toast.success("Member added");
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to add member");
    }
  };

  const handleRemove = async (userId: string) => {
    if (!activeTeam) return;
    try {
      await api.removeTeamMember(activeTeam.id, userId);
      setMembers(members.filter(m => m.user_id !== userId));
      toast.success("Member removed");
    } catch {
      toast.error("Failed to remove member");
    }
  };

  if (!activeTeam) {
    return <EmptyState icon={Users} title="No team selected" description="Select a team from the sidebar" />;
  }

  const initials = (name: string) =>
    name.split(" ").map(n => n[0]).join("").toUpperCase().slice(0, 2);

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-xl font-semibold text-foreground">{activeTeam.name}</h1>
        <p className="text-sm text-muted-foreground mt-1">Team settings and members</p>
      </div>

      <Tabs defaultValue="members">
        <TabsList>
          <TabsTrigger value="members">Members</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        <TabsContent value="members" className="mt-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold text-foreground">
              {members.length} member{members.length !== 1 ? "s" : ""}
            </h2>
            <Dialog open={addOpen} onOpenChange={setAddOpen}>
              <DialogTrigger asChild>
                <Button size="sm">
                  <UserPlus className="mr-2 h-3.5 w-3.5" />
                  Add Member
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Add Team Member</DialogTitle>
                </DialogHeader>
                <div className="space-y-4 pt-4">
                  <div className="space-y-2">
                    <Label htmlFor="member-email">Email</Label>
                    <Input
                      id="member-email"
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="colleague@example.com"
                    />
                  </div>
                  <Button onClick={handleAdd} className="w-full">Add Member</Button>
                </div>
              </DialogContent>
            </Dialog>
          </div>

          {membersError && (
            <div className="mb-4 rounded-lg border border-error/30 bg-error/5 p-4 text-sm text-error">
              {membersError}
            </div>
          )}
          {members.length === 0 && !membersError ? (
            <EmptyState icon={Users} title="No members" description="Add team members to collaborate" />
          ) : (
            <div className="rounded-lg border border-border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>Member</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead className="w-20">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {members.map(m => (
                    <TableRow key={m.user_id}>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <Avatar className="h-7 w-7">
                            <AvatarFallback className="bg-muted text-xs">
                              {initials(m.name || m.email || "?")}
                            </AvatarFallback>
                          </Avatar>
                          <div>
                            <p className="text-sm font-medium">{m.name || m.email || m.user_id}</p>
                            {m.email && m.name && (
                              <p className="text-xs text-muted-foreground">{m.email}</p>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-xs">{m.role}</Badge>
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          onClick={() => handleRemove(m.user_id)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        <TabsContent value="settings" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Team Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label className="text-muted-foreground">Team Name</Label>
                <p className="text-sm font-medium mt-1">{activeTeam.name}</p>
              </div>
              <div>
                <Label className="text-muted-foreground">Slug</Label>
                <p className="text-sm font-mono text-muted-foreground mt-1">{activeTeam.slug}</p>
              </div>
              <div>
                <Label className="text-muted-foreground">Team ID</Label>
                <p className="text-xs font-mono text-muted-foreground mt-1">{activeTeam.id}</p>
              </div>
              <Separator />
              <div>
                <h3 className="text-sm font-semibold text-destructive mb-2">Danger Zone</h3>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={async () => {
                    if (!confirm(`Delete team "${activeTeam.name}"? This cannot be undone.`)) return;
                    try {
                      await api.deleteTeam(activeTeam.id);
                      toast.success("Team deleted");
                      refreshTeams();
                      navigate("/app");
                    } catch (err: unknown) {
                      toast.error(err instanceof Error ? err.message : "Failed to delete team");
                    }
                  }}
                >
                  Delete Team
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
