import * as React from "react";
import { CardHeader, CardDescription, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type SectionHeaderProps = {
  title: string;
  description?: string;
  className?: string;
};

export default function SectionHeader({ title, description, className }: SectionHeaderProps) {
  return (
    <CardHeader className={cn(className)}>
      <CardTitle>{title}</CardTitle>
      {description && <CardDescription>{description}</CardDescription>}
    </CardHeader>
  );
}
