import React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ShieldAlert, ShieldCheck, Zap, Activity, Github } from "lucide-react";

export default function OverviewPage() {
  // Mock data for demonstration
  const stats = [
    { label: "Total Repos", value: "12", icon: <Github />, color: "text-blue-400" },
    { label: "Active Analysis", value: "1,284", icon: <Zap />, color: "text-yellow-400" },
    { label: "Findings Fixed", value: "452", icon: <ShieldCheck />, color: "text-green-400" },
    { label: "Critical Issues", value: "12", icon: <ShieldAlert />, color: "text-red-400" },
  ];

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">System Overview</h1>
        <p className="text-slate-400">Real-time monitoring of your codebase health across all organizations.</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((stat) => (
          <Card key={stat.label} className="bg-slate-900 border-slate-800">
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-slate-400">{stat.label}</CardTitle>
              <div className={stat.color}>{stat.icon}</div>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stat.value}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Recent Activity */}
        <Card className="lg:col-span-2 bg-slate-900 border-slate-800">
          <CardHeader>
            <CardTitle className="text-lg">Recent Analysis Activity</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {[
                { repo: "vianra/core-engine", event: "Push", status: "Success", findings: 2, time: "2m ago" },
                { repo: "vianra/api-gateway", event: "PR #42", status: "Failed", findings: 5, time: "15m ago" },
                { repo: "vianra/ui-kit", event: "Push", status: "Success", findings: 0, time: "1h ago" },
                { repo: "vianra/auth-svc", event: "PR #11", status: "Success", findings: 1, time: "3h ago" },
              ].map((item, i) => (
                <div key={i} className="flex items-center justify-between p-3 rounded-lg bg-slate-800/50 border border-slate-700/50">
                  <div className="flex items-center gap-3">
                    <div className={`w-2 h-2 rounded-full ${item.status === 'Success' ? 'bg-green-500' : 'bg-red-500'}`} />
                    <span className="font-medium">{item.repo}</span>
                    <Badge variant="outline" className="text-xs">{item.event}</Badge>
                  </div>
                  <div className="flex items-center gap-4 text-sm text-slate-400">
                    <span>{item.findings} findings</span>
                    <span>{item.time}</span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Fleet Health */}
        <Card className="bg-slate-900 border-slate-800">
          <CardHeader>
            <CardTitle className="text-lg">Fleet Health</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col items-center justify-center py-8">
             <div className="relative w-32 h-32">
               {/* Mock circular progress */}
               <svg className="w-full h-full" viewBox="0 0 36 36">
                 <path className="text-slate-800" strokeWidth="3" stroke="currentColor" fill="transparent" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831" />
                 <path className="text-green-500" strokeWidth="3" strokeLinecap="round" stroke="currentColor" fill="transparent" d="M18 2.0845 a 15.9155 15.9155 0 0 1 12 10" />
               </svg>
               <div className="absolute inset-0 flex items-center justify-center text-2xl font-bold">
                 84%
               </div>
             </div>
             <p className="mt-4 text-sm text-slate-400 text-center">
               Average health score across all active repositories.
             </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}