import { Metadata } from "next";
import HomeClient from "./HomeClient";

export const metadata: Metadata = {
  title: "Code Search",
  description: "Search across all your repositories with Zoekt. Fast, precise, and powerful code search.",
};

export default function Home() {
  return <HomeClient />;
}
