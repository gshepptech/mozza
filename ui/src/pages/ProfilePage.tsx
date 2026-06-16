import { useState } from "react";
import { useAuth } from "../context/AuthContext";
import * as api from "../api/client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { toast } from "sonner";

export default function ProfilePage() {
  const { user, refreshUser } = useAuth();
  const [name, setName] = useState(user?.name || "");
  const [editing, setEditing] = useState(false);

  if (!user) return null;

  const initials = user.name
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  const handleSave = async () => {
    try {
      await api.updateProfile(name);
      await refreshUser();
      setEditing(false);
      toast.success("Profile updated");
    } catch {
      toast.error("Failed to update profile");
    }
  };

  return (
    <div>
      <h1 className="text-xl font-semibold text-foreground mb-8">Profile</h1>

      <div className="max-w-lg space-y-6">
        {/* Avatar + Name */}
        <Card className="oven-card">
          <CardContent className="flex items-center gap-6 pt-6">
            <div className="relative">
              <div className="absolute inset-0 rounded-full bg-brand/20 blur-lg scale-125" />
              <Avatar className="relative h-16 w-16 ring-2 ring-brand/30">
                <AvatarFallback className="bg-brand text-primary-foreground text-xl font-bold">
                  {initials}
                </AvatarFallback>
              </Avatar>
            </div>
            <div>
              <p className="text-lg font-semibold text-foreground">{user.name}</p>
              <p className="text-sm text-muted-foreground">{user.email}</p>
              <Badge variant="outline" className="mt-1 text-xs border-brand/30 text-brand">{user.role}</Badge>
            </div>
          </CardContent>
        </Card>

        {/* Editable fields */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Account Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="profile-name">Name</Label>
              {editing ? (
                <div className="flex gap-2">
                  <Input
                    id="profile-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                  <Button size="sm" onClick={handleSave}>Save</Button>
                  <Button size="sm" variant="outline" onClick={() => { setEditing(false); setName(user.name); }}>Cancel</Button>
                </div>
              ) : (
                <div className="flex items-center justify-between">
                  <p className="text-sm">{user.name}</p>
                  <Button variant="outline" size="sm" onClick={() => setEditing(true)}>Edit</Button>
                </div>
              )}
            </div>

            <Separator />

            <div className="space-y-2">
              <Label>Email</Label>
              <p className="text-sm text-muted-foreground">{user.email}</p>
            </div>

            <div className="space-y-2">
              <Label>Role</Label>
              <p className="text-sm text-muted-foreground">{user.role}</p>
            </div>

            <div className="space-y-2">
              <Label>User ID</Label>
              <p className="text-xs font-mono text-muted-foreground">{user.id}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
