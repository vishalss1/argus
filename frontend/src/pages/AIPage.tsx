import React, { useEffect, useState } from "react";
import { api } from "../services/api";
import { SemanticEvent, Incident } from "../types/api";
import { Card, CardHeader, CardTitle, CardContent } from "../components/ui";

const AIPage: React.FC = () => {
  const [events, setEvents] = useState<SemanticEvent[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [evs, incs] = await Promise.all([
          api.ai.listEvents(),
          api.ai.listIncidents()
        ]);
        setEvents(evs);
        setIncidents(incs);
      } catch (err) {
        console.error("Failed to fetch AI data", err);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  if (loading) {
    return <div className="p-8">Loading AI insights...</div>;
  }

  return (
    <div className="p-8 space-y-8">
      <h1 className="text-3xl font-bold">AI Runtime Insights</h1>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-red-500">Active Incidents</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {incidents.filter(i => i.status === "open").map(inc => (
            <Card key={inc.id} className="border-red-200 bg-red-50">
              <CardHeader>
                <CardTitle className="text-red-700">{inc.title}</CardTitle>
                <div className="text-sm text-red-500 uppercase font-bold">{inc.severity}</div>
              </CardHeader>
              <CardContent>
                <p className="text-red-600">{inc.summary}</p>
                <div className="mt-4 text-xs text-red-400">
                  Started: {new Date(inc.started_at).toLocaleString()}
                </div>
              </CardContent>
            </Card>
          ))}
          {incidents.filter(i => i.status === "open").length === 0 && (
            <p className="text-gray-500 italic">No active incidents detected.</p>
          )}
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold">Semantic Operational Events</h2>
        <div className="bg-white border rounded-lg overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Time</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Device</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Type</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Severity</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Summary</th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {events.map(ev => (
                <tr key={ev.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {new Date(ev.created_at).toLocaleTimeString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {ev.device_id}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {ev.type}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${
                      ev.severity === "critical" ? "bg-red-100 text-red-800" : 
                      ev.severity === "warning" ? "bg-yellow-100 text-yellow-800" : 
                      "bg-blue-100 text-blue-800"
                    }`}>
                      {ev.severity}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {ev.summary}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
};

export default AIPage;
