import { Metadata } from "next";
import ReposClient from "./ReposClient";

export const metadata: Metadata = {
  title: "Repositories | Code Search",
  description: "Manage indexed repositories, configure branches, and trigger sync operations.",
};

export default function ReposPage() {
  return <ReposClient />;
}
