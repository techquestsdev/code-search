import { Metadata } from "next";
import ConnectionsClient from "./ConnectionsClient";

export const metadata: Metadata = {
  title: "Connections | Code Search",
  description: "Configure and manage connections to code hosts like GitHub, GitLab, and Bitbucket.",
};

export default function ConnectionsPage() {
  return <ConnectionsClient />;
}
