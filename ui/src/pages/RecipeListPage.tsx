import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useDashboard } from "./DashboardPage";
import * as api from "../api/client";
import type { Recipe } from "../api/types";
import { Plus, FileText, MoreHorizontal, Trash2, Pencil } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/custom/empty-state";
import { toast } from "sonner";

export default function RecipeListPage() {
  const { activeTeam } = useDashboard();
  const navigate = useNavigate();
  const [recipes, setRecipes] = useState<Recipe[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!activeTeam) return;
    setLoading(true);
    setError(null);
    api.listRecipes(activeTeam.id)
      .then(({ recipes }) => setRecipes(recipes))
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load recipes");
      })
      .finally(() => setLoading(false));
  }, [activeTeam?.id]);

  const handleNewRecipe = async () => {
    if (!activeTeam) return;
    try {
      const recipe = await api.createRecipe(activeTeam.id, "Untitled Recipe", "", "{}");
      navigate(`/app/recipes/${recipe.id}`);
    } catch {
      toast.error("Failed to create recipe");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteRecipe(id);
      setRecipes(recipes.filter(r => r.id !== id));
      toast.success("Recipe deleted");
    } catch {
      toast.error("Failed to delete recipe");
    }
  };

  if (!activeTeam) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <EmptyState
          icon={FileText}
          title="No team selected"
          description="Create a team to get started with recipes"
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
          <h1 className="text-2xl font-bold text-foreground tracking-tight">Recipes</h1>
          <p className="text-sm text-muted-foreground mt-1.5">{activeTeam.name}</p>
        </div>
        <Button
          onClick={handleNewRecipe}
          className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.15)]"
        >
          <Plus className="mr-2 h-4 w-4" />
          New Recipe
        </Button>
      </div>

      {/* Divider */}
      <div className="pizza-cutter mb-10" />

      {error && (
        <div className="mb-6 rounded-lg border border-error/30 bg-error/5 p-4 text-sm text-error">
          {error}
        </div>
      )}

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map(i => (
            <Card key={i} className="oven-card bg-card border-border/50">
              <CardHeader className="p-6 pb-3"><Skeleton className="h-5 w-32" /></CardHeader>
              <CardContent className="px-6 pb-6"><Skeleton className="h-24 w-full rounded-lg" /></CardContent>
            </Card>
          ))}
        </div>
      ) : recipes.length === 0 ? (
        <EmptyState
          icon={FileText}
          title="No recipes yet"
          description="Create your first recipe to define your application stack"
          action={{ label: "Create Recipe", onClick: handleNewRecipe }}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {recipes.map(recipe => (
            <Card
              key={recipe.id}
              className="oven-card feature-card bg-card border-border/50 cursor-pointer"
              onClick={() => navigate(`/app/recipes/${recipe.id}`)}
            >
              <CardHeader className="flex flex-row items-center justify-between space-y-0 p-6 pb-3">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-xl bg-brand/10 border border-brand/15">
                    <FileText className="h-4 w-4 text-brand" />
                  </div>
                  <CardTitle className="text-sm font-bold">{recipe.name}</CardTitle>
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                    <Button variant="ghost" size="icon" className="h-8 w-8">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={(e) => { e.stopPropagation(); navigate(`/app/recipes/${recipe.id}`); }}>
                      <Pencil className="mr-2 h-3.5 w-3.5" /> Edit
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      className="text-destructive"
                      onClick={(e) => { e.stopPropagation(); handleDelete(recipe.id); }}
                    >
                      <Trash2 className="mr-2 h-3.5 w-3.5" /> Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </CardHeader>
              <CardContent className="px-6 pb-4">
                <pre className="text-xs font-mono text-code-text leading-relaxed max-h-24 overflow-hidden rounded-lg bg-code-bg border border-border/30 p-4">
                  {recipe.source || "Empty recipe"}
                </pre>
              </CardContent>
              <CardFooter className="border-t border-border/40 px-6 py-4">
                <span className="text-xs text-muted-foreground font-mono">{recipe.id.slice(0, 8)}</span>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
