import { useMemo } from 'react'
import type { TerminalResourceSnapshot } from '../../hooks/useTerminal'
import type { HarnessExecutionSummary } from '../../types/harness'

export type ResourceMonitorProps = {
  resources: TerminalResourceSnapshot
  activeExecutions?: HarnessExecutionSummary[]
}

export function ResourceMonitor({ resources, activeExecutions = [] }: ResourceMonitorProps) {
  const aggregated = useMemo(() => {
    if (activeExecutions.length === 0) {
      return {
        memoryMB: resources.memoryMB,
        memoryLimitMB: resources.memoryLimitMB || 2400,
        cpuPriority: resources.cpuPriority,
        cpuLimit: resources.cpuLimit || 4,
        leases: resources.activeLeases,
      }
    }
    const mem = activeExecutions.reduce((acc, curr) => acc + curr.memoryMB, 0)
    const cpu = activeExecutions.reduce((acc, curr) => acc + curr.cpuPriority, 0)
    return {
      memoryMB: mem > 0 ? mem : resources.memoryMB,
      memoryLimitMB: resources.memoryLimitMB || 2400,
      cpuPriority: cpu > 0 ? cpu : resources.cpuPriority,
      cpuLimit: resources.cpuLimit || 4,
      leases: activeExecutions.length,
    }
  }, [activeExecutions, resources])

  const memPct = Math.min(100, Math.round((aggregated.memoryMB / (aggregated.memoryLimitMB || 1)) * 100))
  const cpuPct = Math.min(100, Math.round((aggregated.cpuPriority / (aggregated.cpuLimit || 1)) * 100))

  const memStatus = memPct > 85 ? 'danger' : memPct > 65 ? 'warning' : 'good'
  const cpuStatus = cpuPct > 85 ? 'danger' : cpuPct > 65 ? 'warning' : 'good'

  return (
    <section className="terminal-resource-monitor" aria-labelledby="resource-monitor-title">
      <div className="resource-header">
        <div>
          <span className="eyebrow">HARNESS GOVERNOR</span>
          <h3 id="resource-monitor-title">Resource & Lease Monitor</h3>
        </div>
        <span className="resource-badge">Governed Sandbox</span>
      </div>

      <div className="resource-gauges">
        <div className={`gauge-card ${memStatus}`}>
          <div className="gauge-label">
            <span>Memory Lease</span>
            <strong>{aggregated.memoryMB} / {aggregated.memoryLimitMB} MB</strong>
          </div>
          <div className="gauge-bar-track" role="progressbar" aria-valuenow={memPct} aria-valuemin={0} aria-valuemax={100} aria-label="Memory lease usage">
            <div className={`gauge-bar-fill ${memStatus}`} style={{ width: `${memPct}%` }} />
          </div>
          <div className="gauge-footer">
            <span className="gauge-pct">{memPct}% of budget</span>
            <span className={`gauge-status-dot ${memStatus}`} />
          </div>
        </div>

        <div className={`gauge-card ${cpuStatus}`}>
          <div className="gauge-label">
            <span>CPU Priority Slots</span>
            <strong>{aggregated.cpuPriority} / {aggregated.cpuLimit}</strong>
          </div>
          <div className="gauge-bar-track" role="progressbar" aria-valuenow={cpuPct} aria-valuemin={0} aria-valuemax={100} aria-label="CPU priority slots usage">
            <div className={`gauge-bar-fill ${cpuStatus}`} style={{ width: `${cpuPct}%` }} />
          </div>
          <div className="gauge-footer">
            <span className="gauge-pct">{cpuPct}% allocated</span>
            <span className={`gauge-status-dot ${cpuStatus}`} />
          </div>
        </div>

        <div className="gauge-card neutral">
          <div className="gauge-label">
            <span>Active Worker Leases</span>
            <strong>{aggregated.leases}</strong>
          </div>
          <div className="lease-metric">
            <span className="lease-metric-value">{aggregated.leases}</span>
            <span className="lease-metric-unit">concurrent execution slots</span>
          </div>
          <div className="gauge-footer">
            <span className="gauge-pct">ADR-0003 worker pool</span>
          </div>
        </div>
      </div>

      {activeExecutions.length > 0 && (
        <div className="active-leases-table-wrap">
          <h4>Active Execution Leases</h4>
          <table className="active-leases-table">
            <thead>
              <tr>
                <th>Execution ID</th>
                <th>Task</th>
                <th>Class</th>
                <th>Memory</th>
                <th>CPU</th>
                <th>Worker</th>
              </tr>
            </thead>
            <tbody>
              {activeExecutions.map((exec) => (
                <tr key={exec.id}>
                  <td><code>{exec.id}</code></td>
                  <td><code>{exec.taskId}</code></td>
                  <td><span className="badge-class">{exec.engineClass}</span></td>
                  <td>{exec.memoryMB} MB</td>
                  <td>P{exec.cpuPriority}</td>
                  <td>{exec.workerId || 'pending'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
