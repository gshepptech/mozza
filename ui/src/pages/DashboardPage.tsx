import { useState, useEffect, useCallback } from "react";
import { Outlet, Link, useNavigate, useLocation, useOutletContext } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { ClusterStatusProvider } from "../context/ClusterContext";
import * as api from "../api/client";
import type { Team } from "../api/types";
import {
  LayoutDashboard, Box, Rocket, Layers, Activity, Stethoscope, Server,
  Users, User, LogOut, ChevronsLeft, ChevronsRight,
  Search, Plus, FileText, Play, Store,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator,
} from "@/components/ui/command";
import { cn } from "@/lib/utils";
import { MozzaLogo } from "@/components/custom/MozzaLogo";
import { PageBackground } from "@/components/custom/PageBackground";

const workspaceNav = [
  { path: "/app", label: "Overview", icon: LayoutDashboard },
  { path: "/app/applications", label: "Applications", icon: Box },
  { path: "/app/deploy", label: "Deploy", icon: Play },
  { path: "/app/deployments", label: "Deployments", icon: Rocket },
  { path: "/app/environments", label: "Environments", icon: Layers },
  { path: "/app/monitoring", label: "Monitoring", icon: Activity },
  { path: "/app/clusters", label: "Clusters", icon: Server },
  { path: "/app/doctor", label: "Doctor", icon: Stethoscope },
  { path: "/app/recipes", label: "Recipes", icon: FileText },
  { path: "/app/marketplace", label: "Marketplace", icon: Store },
];

const settingsNav = [
  { path: "/app/teams/", label: "Team", icon: Users, dynamic: true },
  { path: "/app/profile", label: "Profile", icon: User },
];

export default function DashboardPage() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [teams, setTeams] = useState<Team[]>([]);
  const [activeTeam, setActiveTeam] = useState<Team | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [cmdOpen, setCmdOpen] = useState(false);

  const [teamsError, setTeamsError] = useState<string | null>(null);

  const refreshTeams = useCallback(() => {
    setTeamsError(null);
    api.listTeams().then(({ teams }) => {
      setTeams(teams);
      if (teams.length > 0 && !activeTeam) {
        setActiveTeam(teams[0] ?? null);
      }
    }).catch((err) => {
      setTeamsError(err instanceof Error ? err.message : "Failed to load teams");
    });
  }, [activeTeam]);

  useEffect(() => {
    refreshTeams();
  }, []);

  // Cmd+K shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setCmdOpen((prev) => !prev);
      }
      if (e.key === "b" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setCollapsed((prev) => !prev);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  const handleLogout = useCallback(async () => {
    await logout();
    navigate("/login");
  }, [logout, navigate]);

  const isActive = (path: string, dynamic?: boolean) => {
    if (dynamic) return location.pathname.startsWith(path);
    return location.pathname === path;
  };

  const getTeamPath = () => activeTeam ? "/app/teams/" + activeTeam.id : "/app/teams/new";

  const initials = user?.name
    ?.split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2) || "?";

  return (
    <div className="grain-overlay flex min-h-screen bg-background text-foreground">
      {/* Sidebar */}
      <aside
        className={cn(
          "sidebar-glow flex flex-col transition-all duration-200",
          collapsed ? "w-16" : "w-60"
        )}
      >
        {/* Logo */}
        <div className="flex items-center justify-between px-4 pt-5 pb-4">
          {!collapsed && (
            <Link to="/app" className="flex items-center gap-3 group">
              <MozzaLogo className="text-brand transition-transform group-hover:scale-110 drop-shadow-[0_0_8px_rgba(255,107,53,0.3)]" size={28} />
              <span className="text-lg font-extrabold text-foreground tracking-tight">Mozza</span>
            </Link>
          )}
          {collapsed && (
            <Link to="/app" className="mx-auto group">
              <MozzaLogo className="text-brand transition-transform group-hover:scale-110 drop-shadow-[0_0_8px_rgba(255,107,53,0.3)]" size={26} />
            </Link>
          )}
          {!collapsed && (
            <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground/40 hover:text-foreground" onClick={() => setCollapsed(true)}>
              <ChevronsLeft className="h-4 w-4" />
            </Button>
          )}
        </div>

        {/* Team selector */}
        {!collapsed && (
          <div className="px-3 pb-3">
            {teams.length > 0 ? (
              <div className="flex gap-1.5">
                <Select
                  value={activeTeam?.id || ""}
                  onValueChange={(val) => {
                    const t = teams.find((t) => t.id === val);
                    if (t) setActiveTeam(t);
                  }}
                >
                  <SelectTrigger className="h-9 text-sm bg-white/[0.03] border-white/[0.06] flex-1 rounded-lg">
                    <SelectValue placeholder="Select team" />
                  </SelectTrigger>
                  <SelectContent>
                    {teams.map((t) => (
                      <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="icon"
                      className="h-9 w-9 shrink-0 rounded-lg border-dashed border-brand/25 text-brand hover:bg-brand/10"
                      onClick={() => navigate("/app/teams/new")}
                    >
                      <Plus className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>New team</TooltipContent>
                </Tooltip>
              </div>
            ) : (
              <Button
                variant="outline"
                className="h-9 w-full text-sm rounded-lg border-dashed border-brand/25 text-brand hover:bg-brand/10"
                onClick={() => navigate("/app/teams/new")}
              >
                <Plus className="mr-2 h-4 w-4" />
                Create a team
              </Button>
            )}
          </div>
        )}

        {/* Search */}
        {!collapsed && (
          <div className="px-3 pb-3">
            <button
              className="flex items-center gap-2.5 w-full h-9 px-3 rounded-lg text-sm text-muted-foreground/50 bg-white/[0.03] hover:bg-white/[0.06] transition-colors"
              onClick={() => setCmdOpen(true)}
            >
              <Search className="h-4 w-4" />
              <span>Search...</span>
              <kbd className="ml-auto text-[10px] font-mono text-muted-foreground/30">
                {"\u2318"}K
              </kbd>
            </button>
          </div>
        )}

        {/* Nav */}
        <nav className="flex-1 px-3 py-1 space-y-1">
          {workspaceNav.map((item) => {
            const active = isActive(item.path);
            const link = (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition-all",
                  active
                    ? "bg-brand/12 text-brand shadow-[inset_0_1px_0_rgba(255,107,53,0.1)]"
                    : "text-muted-foreground hover:bg-white/[0.05] hover:text-foreground",
                  collapsed && "justify-center px-0 rounded-lg"
                )}
              >
                <item.icon className={cn("h-[18px] w-[18px] shrink-0", active ? "text-brand" : "text-muted-foreground/70")} />
                {!collapsed && <span>{item.label}</span>}
              </Link>
            );
            if (collapsed) {
              return (
                <Tooltip key={item.path}>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              );
            }
            return link;
          })}

          <div className="my-3 mx-1 h-px bg-white/[0.05]" />

          {settingsNav.map((item) => {
            const path = item.dynamic ? getTeamPath() : item.path;
            const active = isActive(item.dynamic ? "/app/teams/" : item.path, item.dynamic);
            const link = (
              <Link
                key={item.label}
                to={path}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition-all",
                  active
                    ? "bg-brand/12 text-brand shadow-[inset_0_1px_0_rgba(255,107,53,0.1)]"
                    : "text-muted-foreground hover:bg-white/[0.05] hover:text-foreground",
                  collapsed && "justify-center px-0 rounded-lg"
                )}
              >
                <item.icon className={cn("h-[18px] w-[18px] shrink-0", active ? "text-brand" : "text-muted-foreground/70")} />
                {!collapsed && <span>{item.label}</span>}
              </Link>
            );
            if (collapsed) {
              return (
                <Tooltip key={item.label}>
                  <TooltipTrigger asChild>{link}</TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              );
            }
            return link;
          })}
        </nav>

        {/* User */}
        <div className="flex items-center gap-3 px-4 py-3 border-t border-white/[0.05]">
          <Avatar className="h-8 w-8">
            <AvatarFallback className="bg-brand/12 text-brand text-xs font-bold">{initials}</AvatarFallback>
          </Avatar>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <p className="truncate text-sm font-medium text-foreground">{user?.name}</p>
            </div>
          )}
          {!collapsed ? (
            <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground/40 hover:text-foreground" onClick={handleLogout}>
              <LogOut className="h-4 w-4" />
            </Button>
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground/40 hover:text-foreground" onClick={() => setCollapsed(false)}>
                  <ChevronsRight className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right">Expand sidebar</TooltipContent>
            </Tooltip>
          )}
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto p-8 relative">
        <PageBackground />
        {teamsError && (
          <div className="mb-4 rounded-lg border border-error/30 bg-error/5 p-3 text-sm text-error relative z-10">
            {teamsError}
          </div>
        )}
        <div className="page-enter relative z-10" key={location.pathname}>
          <ClusterStatusProvider>
            <Outlet context={{ activeTeam, teams, setActiveTeam, refreshTeams }} />
          </ClusterStatusProvider>
        </div>
      </main>

      {/* Command palette */}
      <CommandDialog open={cmdOpen} onOpenChange={setCmdOpen}>
        <CommandInput placeholder="Type a command or search..." />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          <CommandGroup heading="Navigation">
            {workspaceNav.map((item) => (
              <CommandItem
                key={item.path}
                onSelect={() => { navigate(item.path); setCmdOpen(false); }}
              >
                <item.icon className="mr-2 h-4 w-4" />
                {item.label}
              </CommandItem>
            ))}
            <CommandItem onSelect={() => { navigate(getTeamPath()); setCmdOpen(false); }}>
              <Users className="mr-2 h-4 w-4" />
              Team Settings
            </CommandItem>
            <CommandItem onSelect={() => { navigate("/app/profile"); setCmdOpen(false); }}>
              <User className="mr-2 h-4 w-4" />
              Profile
            </CommandItem>
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="Actions">
            <CommandItem onSelect={() => { navigate("/app/applications"); setCmdOpen(false); }}>
              <Box className="mr-2 h-4 w-4" />
              New Application
            </CommandItem>
            <CommandItem onSelect={() => { navigate("/app/doctor"); setCmdOpen(false); }}>
              <Stethoscope className="mr-2 h-4 w-4" />
              Run Doctor Check
            </CommandItem>
            <CommandItem onSelect={() => { handleLogout(); setCmdOpen(false); }}>
              <LogOut className="mr-2 h-4 w-4" />
              Sign Out
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </div>
  );
}

// Hook for child routes to access dashboard context.
interface DashboardContext {
  activeTeam: Team | null;
  teams: Team[];
  setActiveTeam: (t: Team) => void;
  refreshTeams: () => void;
}

export function useDashboard(): DashboardContext {
  return useOutletContext<DashboardContext>();
}
