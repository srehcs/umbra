import * as React from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { cn } from "@/lib/utils";

type StatusBannerProps = {
  title: string;
  description: React.ReactNode;
  variant?: "default" | "destructive";
  className?: string;
};

export default function StatusBanner({ title, description, variant = "default", className }: StatusBannerProps) {
  const variantClass =
    variant === "destructive"
      ? "border-destructive/40 bg-destructive/10 text-destructive [&_p]:text-destructive/80"
      : "border-border bg-muted text-foreground";
  return (
    <Alert className={cn(variantClass, className)}>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{description}</AlertDescription>
    </Alert>
  );
}
