import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "Prediction Agent Arena",
  description: "Local simulated prediction-market agent arena",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <main className="shell">
          <header className="topbar">
            <Link className="brand" href="/">
              <strong>Prediction Agent Arena</strong>
              <span>local paper-trading arena control plane</span>
            </Link>
            <nav className="nav">
              <Link href="/">Overview</Link>
              <Link href="/agent">Agent</Link>
              <Link href="/leaderboard">Leaderboard</Link>
              <Link href="/leaderboard/evaluation">Evaluation</Link>
              <Link href="/admin">Admin</Link>
            </nav>
          </header>
          {children}
        </main>
      </body>
    </html>
  );
}
