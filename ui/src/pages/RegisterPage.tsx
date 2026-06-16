import { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { Loader, Mail, Lock, User } from "lucide-react";
import { MozzaLogoLarge } from "@/components/custom/MozzaLogo";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert } from "@/components/ui/alert";

export default function RegisterPage() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await register(email, name, password);
      navigate("/app");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Registration failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen">
      {/* Brand panel */}
      <div className="hidden lg:flex lg:w-1/2 flex-col items-center justify-center bg-surface border-r border-border/40 relative overflow-hidden">
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 w-[500px] h-[500px] rounded-full bg-[radial-gradient(ellipse_60%_50%_at_50%_50%,rgba(255,107,53,0.12)_0%,rgba(255,107,53,0.03)_50%,transparent_70%)] animate-oven-glow" />
        <div className="absolute bottom-1/4 right-1/4 w-64 h-64 rounded-full bg-[radial-gradient(circle,rgba(96,165,250,0.04)_0%,transparent_60%)]" />
        <div className="absolute bottom-0 left-0 right-0 h-1/3 bg-gradient-to-t from-brand/5 to-transparent" />
        <div className="absolute top-0 left-0 right-0 h-px bg-gradient-to-r from-transparent via-brand/20 to-transparent" />
        <MozzaLogoLarge className="relative text-brand mb-8 animate-float" size={80} />
        <h1 className="neon-text relative text-4xl font-extrabold text-brand mb-3 tracking-tight">Mozza</h1>
        <p className="relative text-muted-foreground text-base">Deploy apps like you order pizza</p>
      </div>

      {/* Form panel */}
      <div className="flex flex-1 items-center justify-center p-8 bg-background relative overflow-hidden">
        <div className="absolute top-0 right-0 w-[400px] h-[400px] bg-[radial-gradient(circle,rgba(255,107,53,0.03)_0%,transparent_60%)] pointer-events-none" />
        <div className="w-full max-w-md page-enter">
          <div className="mb-10 lg:hidden text-center">
            <MozzaLogoLarge className="text-brand mx-auto mb-4 animate-float" size={56} />
            <h1 className="neon-text text-3xl font-extrabold text-brand">Mozza</h1>
          </div>

          <div className="oven-card rounded-2xl bg-card border border-border/60 p-8 shadow-[var(--shadow-elevated)]">
            <h2 className="text-2xl font-bold text-foreground tracking-tight mb-1">Create account</h2>
            <p className="text-sm text-muted-foreground mb-8">Get started with Mozza</p>

            {error && (
              <Alert variant="destructive" className="mb-6 text-sm">
                {error}
              </Alert>
            )}

            <form onSubmit={handleSubmit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="name" className="text-sm font-medium">Name</Label>
                <div className="relative">
                  <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    id="name"
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Your name"
                    required
                    className="pl-10 h-11 bg-elevated"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="email" className="text-sm font-medium">Email</Label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    required
                    className="pl-10 h-11 bg-elevated"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="password" className="text-sm font-medium">Password</Label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••"
                    required
                    className="pl-10 h-11 bg-elevated"
                  />
                </div>
              </div>
              <Button
                type="submit"
                disabled={loading}
                className="w-full h-11 bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.2)] font-semibold"
              >
                {loading ? <Loader className="mr-2 h-4 w-4 animate-spin" /> : null}
                {loading ? "Creating account..." : "Create Account"}
              </Button>
            </form>

            <div className="brand-divider my-8" />

            <p className="text-center text-sm text-muted-foreground">
              Already have an account?{" "}
              <Link to="/login" className="text-brand hover:underline font-medium">Sign in</Link>
            </p>
          </div>

          <p className="mt-6 text-center text-sm">
            <Link to="/" className="text-muted-foreground hover:text-foreground">&larr; Back to home</Link>
          </p>
        </div>
      </div>
    </div>
  );
}
