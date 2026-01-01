"use client";

import * as React from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth";

const ROLE_STORAGE_KEY = "umbra.roles";
const USER_STORAGE_KEY = "umbra.user";

export default function AuthDevControls() {
  const { enabled, roles, user } = useAuth();
  const [roleInput, setRoleInput] = React.useState(roles.join(", "));
  const [userInput, setUserInput] = React.useState(user?.id ?? "");

  React.useEffect(() => {
    if (enabled) return;
    setRoleInput(roles.join(", "));
    setUserInput(user?.id ?? "");
  }, [enabled, roles, user]);

  if (enabled) {
    return null;
  }

  function save() {
    const nextRoles = roleInput.trim();
    if (nextRoles) {
      localStorage.setItem(ROLE_STORAGE_KEY, nextRoles);
    } else {
      localStorage.removeItem(ROLE_STORAGE_KEY);
    }
    const nextUser = userInput.trim();
    if (nextUser) {
      localStorage.setItem(USER_STORAGE_KEY, nextUser);
    } else {
      localStorage.removeItem(USER_STORAGE_KEY);
    }
  }

  function clear() {
    localStorage.removeItem(ROLE_STORAGE_KEY);
    localStorage.removeItem(USER_STORAGE_KEY);
    setRoleInput("");
    setUserInput("");
  }

  return (
    <div className="space-y-3 rounded-lg border border-border bg-muted/40 p-4 text-xs">
      <div className="text-xs font-semibold">Dev auth</div>
      <div className="space-y-2">
        <Label className="text-xs">Roles (comma-separated)</Label>
        <Input
          value={roleInput}
          onChange={(e) => setRoleInput(e.target.value)}
          placeholder="policy_admin,tool_admin,auditor"
          className="h-8 text-xs"
        />
      </div>
      <div className="space-y-2">
        <Label className="text-xs">User id</Label>
        <Input
          value={userInput}
          onChange={(e) => setUserInput(e.target.value)}
          placeholder="dev-user"
          className="h-8 text-xs"
        />
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="secondary" onClick={save}>
          Save
        </Button>
        <Button size="sm" variant="outline" onClick={clear}>
          Clear
        </Button>
      </div>
    </div>
  );
}
