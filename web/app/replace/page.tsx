import { Metadata } from "next";
import ReplaceClient from "./ReplaceClient";
import { Suspense } from "react";
import { Loader2 } from "lucide-react";

export const metadata: Metadata = {
  title: "Search & Replace | Code Search",
  description:
    "Search and replace code across multiple repositories with ease.",
};

export default function ReplacePage() {
  return (
    <Suspense
      fallback={
        <div className="mx-auto flex max-w-6xl items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
        </div>
      }
    >
      <ReplaceClient />
    </Suspense>
  );
}
