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
  return (
    <Alert variant={variant} className={cn(className)}>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{description}</AlertDescription>
    </Alert>
  );
}
