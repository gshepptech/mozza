import { useState } from "react";
import * as api from "../../api/client";
import type { ScanResult } from "../../api/types";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { Loader, AlertTriangle, Search, FileCode, Check, Lock, ChevronDown, ChevronRight, Hammer, Copy, RefreshCw } from "lucide-react";

type ScanState = "idle" | "scanning" | "results" | "error";

interface GitHubImportProps {
  onRecipeReady: (source: string) => void;
}

export default function GitHubImport({ onRecipeReady }: GitHubImportProps) {
  const [repoUrl, setRepoUrl] = useState("");
  const [scanState, setScanState] = useState<ScanState>("idle");
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [scanError, setScanError] = useState("");
  const [editing, setEditing] = useState(false);
  const [editedSource, setEditedSource] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [token, setToken] = useState("");
  const [copied, setCopied] = useState(false);
  const [checkingGhcr, setCheckingGhcr] = useState(false);

  const isValidUrl = /^https?:\/\/github\.com\/[\w.-]+\/[\w.-]+/i.test(repoUrl.trim());

  const handleScan = async () => {
    if (!isValidUrl) return;
    setScanState("scanning");
    setScanError("");
    setScanResult(null);
    setEditing(false);
    try {
      const result = await api.scanRepo(repoUrl.trim(), token || undefined);
      setScanResult(result);
      if (result.generated?.source) {
        setEditedSource(result.generated.source);
      }
      setScanState("results");
    } catch (err) {
      const raw = err instanceof Error ? err.message : "";
      const msg = raw.includes("not a GitHub URL") ? "That doesn't look like a GitHub URL. Try something like https://github.com/owner/repo"
        : raw.includes("authentication failed") ? "Authentication failed — check your token has 'repo' scope."
        : raw.includes("404") ? "Repository not found — check the URL and make sure it's public (or provide a token)."
        : raw.includes("rate limit") ? "GitHub API rate limit reached. Try again in a minute, or provide a token."
        : raw || "Failed to scan repository";
      setScanError(msg);
      setScanState("error");
    }
  };

  const handleUseRecipe = () => {
    const source = editing ? editedSource : scanResult?.generated?.source;
    if (source) {
      onRecipeReady(source);
    }
  };

  return (
    <div className="space-y-5">
      {/* URL input */}
      <div className="space-y-2">
        <Label className="text-xs text-muted-foreground font-medium">
          GitHub Repository URL
        </Label>
        <div className="flex gap-2">
          <Input
            value={repoUrl}
            onChange={(e) => setRepoUrl(e.target.value)}
            placeholder="https://github.com/owner/repo"
            className="h-10 text-sm bg-elevated font-mono flex-1"
            onKeyDown={(e) => {
              if (e.key === "Enter" && isValidUrl && scanState !== "scanning") {
                handleScan();
              }
            }}
          />
          <Button
            onClick={handleScan}
            disabled={!isValidUrl || scanState === "scanning"}
            className="bg-brand hover:bg-brand-hover text-primary-foreground shrink-0"
          >
            {scanState === "scanning" ? (
              <Loader className="h-4 w-4 animate-spin" />
            ) : (
              <Search className="h-4 w-4" />
            )}
            <span className="ml-2">Scan</span>
          </Button>
        </div>
        {repoUrl.trim() && !isValidUrl && (
          <p className="text-[10px] text-muted-foreground">
            Enter a full GitHub URL, e.g. https://github.com/owner/repo
          </p>
        )}

        {/* Private repository toggle */}
        <button
          type="button"
          className="flex items-center gap-1.5 text-[11px] text-muted-foreground hover:text-foreground transition-colors mt-1"
          onClick={() => setShowToken(!showToken)}
        >
          <Lock className="h-3 w-3" />
          <span>Private repository?</span>
          {showToken ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        </button>
        {showToken && (
          <div className="space-y-1.5 pt-1">
            <Input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
              className="h-9 text-xs bg-elevated font-mono"
            />
            <p className="text-[10px] text-muted-foreground/70">
              Personal access token with <span className="font-mono text-muted-foreground">repo</span> scope.
              Never stored server-side.
            </p>
            <p className="text-[10px] text-muted-foreground/40 mt-1">
              Only needed for private repos. <a href="https://github.com/settings/tokens/new" target="_blank" rel="noopener noreferrer" className="text-brand hover:underline">Create a token on GitHub</a>
            </p>
          </div>
        )}
      </div>

      {/* Scanning state */}
      {scanState === "scanning" && (
        <div className="flex flex-col items-center py-12 text-center">
          <Loader className="h-6 w-6 text-brand animate-spin mb-3" />
          <p className="text-sm text-foreground font-medium">Scanning repository...</p>
          <p className="text-xs text-muted-foreground mt-1">
            Scanning for deployment files...
          </p>
        </div>
      )}

      {/* Error state */}
      {scanState === "error" && (
        <div className="flex flex-col items-center py-12 text-center">
          <AlertTriangle className="h-6 w-6 text-red-400 mb-3" />
          <p className="text-sm text-red-400 font-medium">{scanError}</p>
          <button
            type="button"
            className="mt-3 text-xs font-medium text-brand hover:underline"
            onClick={handleScan}
          >
            Try again
          </button>
        </div>
      )}

      {/* Results */}
      {scanState === "results" && scanResult && (
        <div className="space-y-4 page-enter">
          {/* Repo info */}
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
              <FileCode className="h-4 w-4 text-muted-foreground" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-bold text-foreground truncate">{scanResult.repo_name}</p>
              {scanResult.description && (
                <p className="text-[11px] text-muted-foreground truncate">{scanResult.description}</p>
              )}
            </div>
          </div>

          {/* Detected sources */}
          {scanResult.sources.length > 0 && (
            <div className="space-y-2">
              <p className="text-[10px] uppercase tracking-wider font-semibold text-muted-foreground/70">
                Detected sources
              </p>
              <div className="flex flex-wrap gap-1.5">
                {scanResult.sources.map((s, i) => (
                  <Badge
                    key={i}
                    variant="outline"
                    className="text-[10px] font-mono px-2 py-0.5"
                  >
                    {s.type} at {s.path}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {/* Warnings */}
          {scanResult.warnings.length > 0 && (
            <div className="space-y-1.5">
              {scanResult.warnings.map((w, i) => (
                <div key={i} className="flex items-start gap-2 text-[11px] text-yellow-400/80">
                  <AlertTriangle className="h-3 w-3 shrink-0 mt-0.5" />
                  <span>{w}</span>
                </div>
              ))}
            </div>
          )}

          {/* Generated recipe */}
          {scanResult.generated && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <p className="text-[10px] uppercase tracking-wider font-semibold text-muted-foreground/70">
                  Generated recipe
                  {scanResult.generated.method && (
                    <span className="ml-2 normal-case tracking-normal text-muted-foreground/50">
                      via {scanResult.generated.method}
                    </span>
                  )}
                </p>
                {scanResult.generated.editable && !editing && (
                  <button
                    type="button"
                    className="text-[10px] font-medium text-brand hover:underline"
                    onClick={() => setEditing(true)}
                  >
                    Edit
                  </button>
                )}
              </div>

              {editing ? (
                <textarea
                  value={editedSource}
                  onChange={(e) => setEditedSource(e.target.value)}
                  className={cn(
                    "w-full min-h-[180px] rounded-lg border border-border/50 bg-elevated p-4",
                    "text-xs font-mono text-foreground leading-relaxed",
                    "focus:outline-none focus:ring-1 focus:ring-brand/40 focus:border-brand/30",
                    "resize-y",
                  )}
                />
              ) : (
                <Card className="bg-elevated border-border/30">
                  <CardContent className="p-4">
                    <pre className="text-xs font-mono text-foreground/90 leading-relaxed whitespace-pre-wrap">
                      {scanResult.generated.source}
                    </pre>
                  </CardContent>
                </Card>
              )}

              {/* Build-from-source guidance */}
              {scanResult.generated.needs_build && scanResult.generated.build_instructions && (
                <Card className="bg-yellow-500/5 border-yellow-500/20">
                  <CardContent className="p-4 space-y-3">
                    <div className="flex items-center gap-2">
                      <Hammer className="h-4 w-4 text-yellow-400 shrink-0" />
                      <p className="text-xs font-semibold text-yellow-400">
                        This repo needs to be built first
                      </p>
                    </div>
                    <div className="relative group">
                      <pre className="text-[11px] font-mono text-foreground/80 leading-relaxed whitespace-pre-wrap bg-black/20 rounded-md p-3 pr-10">
                        {scanResult.generated.build_instructions}
                      </pre>
                      <button
                        type="button"
                        className="absolute top-2 right-2 p-1 rounded text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                        onClick={() => {
                          navigator.clipboard.writeText(scanResult.generated?.build_instructions ?? "");
                          setCopied(true);
                          setTimeout(() => setCopied(false), 2000);
                        }}
                      >
                        {copied ? <Check className="h-3.5 w-3.5 text-green-400" /> : <Copy className="h-3.5 w-3.5" />}
                      </button>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs border-yellow-500/20 hover:bg-yellow-500/10 text-yellow-400"
                      disabled={checkingGhcr}
                      onClick={async () => {
                        setCheckingGhcr(true);
                        try {
                          const fresh = await api.scanRepo(repoUrl.trim(), token || undefined);
                          setScanResult(fresh);
                          if (fresh.generated?.source) setEditedSource(fresh.generated.source);
                        } catch {
                          // non-critical — keep current result
                        } finally {
                          setCheckingGhcr(false);
                        }
                      }}
                    >
                      {checkingGhcr ? (
                        <Loader className="h-3 w-3 animate-spin mr-1.5" />
                      ) : (
                        <RefreshCw className="h-3 w-3 mr-1.5" />
                      )}
                      Check for Pre-built Image
                    </Button>
                  </CardContent>
                </Card>
              )}

              <Button
                onClick={handleUseRecipe}
                className="w-full bg-brand hover:bg-brand-hover text-primary-foreground"
                disabled={editing && !editedSource.trim()}
              >
                <Check className="mr-2 h-4 w-4" />
                Use This Recipe
              </Button>
            </div>
          )}

          {/* No recipe generated */}
          {!scanResult.generated && scanResult.sources.length === 0 && (
            <div className="flex flex-col items-center py-6 text-center">
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">We didn't find deployment files in this repo.</p>
                <p className="text-xs text-muted-foreground/60">
                  This usually means your app hasn't been set up for deployment yet.
                  If you have a Docker image already, use the "Your menu" tab. If not,
                  ask your developer to add a Dockerfile to the repo.
                </p>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
