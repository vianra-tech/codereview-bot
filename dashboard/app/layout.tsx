import React from 'react';
import Link from "next/link";
import { LayoutDashboard, Github, ShieldCheck, Activity, CreditCard, Settings } from "lucide-react";
import "./globals.css";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen bg-slate-950 text-slate-50">
      {/* Sidebar */}
      <aside className="w-64 border-r border-slate-800 bg-slate-900/50 flex flex-col">
        <div className="p-6 flex items-center gap-3">
          <div className="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center font-bold text-slate-900">CR</div>
          <span className="font-bold text-xl tracking-tight">CodeReview.ai</span>
        </div>

        <nav className="flex-1 px-4 space-y-2 mt-4">
          <NavItem href="/" icon={<LayoutDashboard size={20} />} label="Overview" active />
          <NavItem href="/orgs" icon={<Github size={20} />} label="Organizations" />
          <NavItem href="/analytics" icon={<Activity size={20} />} label="Analytics" />
          <NavItem href="/billing" icon={<CreditCard size={20} />} label="Billing" />
          <NavItem href="/settings" icon={<Settings size={20} />} label="Settings" />
        </nav>

        <div className="p-4 border-t border-slate-800">
          <div className="flex items-center gap-3 p-2 rounded-lg bg-slate-800/50">
            <div className="w-8 h-8 rounded-full bg-slate-600" />
            <div className="flex-1 overflow-hidden">
              <p className="text-sm font-medium truncate">Vishal Sodmise</p>
              <p className="text-xs text-slate-400 truncate">Admin</p>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-y-auto p-8">
        {children}
      </main>
    </div>
  );
}

function NavItem({ href, icon, label, active = false }: { href: string; icon: React.ReactNode; label: string; active?: boolean }) {
  return (
    <Link 
      href={href} 
      className={`flex items-center gap-3 px-3 py-2 rounded-md transition-colors ${
        active ? "bg-yellow-500/10 text-yellow-500" : "text-slate-400 hover:bg-slate-800 hover:text-slate-200"
      }`}
    >
      {icon}
      <span className="text-sm font-medium">{label}</span>
    </Link>
  );
}