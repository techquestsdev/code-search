import { Metadata } from "next";
import JobsClient from "./JobsClient";

export const metadata: Metadata = {
  title: "Jobs | Code Search",
  description: "View and manage background jobs for repository indexing and search operations.",
};

export default function JobsPage() {
  return <JobsClient />;
}
