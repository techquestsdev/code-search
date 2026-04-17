import { Metadata } from "next";
import BrowseClient from "./BrowseClient";
import { Suspense } from "react";
import { Loader2 } from "lucide-react";

export const metadata: Metadata = {
  title: "Browse | Code Search",
  description:
    "Browse repository files, view code with syntax highlighting, and navigate symbols.",
};

export default function BrowsePage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
        </div>
      }
    >
      <BrowseClient />
    </Suspense>
  );
}
