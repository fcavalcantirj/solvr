import { CAPTION } from "@/components/page/caption";
import { AGENTS, AGENT_VERSIONS, RUNS, RUNS_BUILD, RUNS_DATE, RUNS_HOW } from "@/lib/docs/guide-runs";
import type { AgentName, WorkflowGuide } from "@/lib/docs/guide-types";
import { GuideText } from "./guide-blocks";

// The run record every guide carries (SPEC.md 27.5): the agents that were run for it and
// their versions, every agent that was not and why, the date, the build, how they were run,
// the backend guide tests that run it, and what was observed. Rendered on the server.

const LABEL = `${CAPTION} sm:pt-1`;

const version = (agent: AgentName) => AGENT_VERSIONS[agent as keyof typeof AGENT_VERSIONS] ?? agent;

export function RunRecord({ guide }: { guide: WorkflowGuide }) {
  const { runs, tests, notRun, observed } = guide.record;
  const notRunAgents = AGENTS.filter((agent) => notRun[agent] !== undefined);

  return (
    <section aria-labelledby="run-record" data-testid="run-record" className="border border-border p-5 sm:p-7">
      <h2 id="run-record" className="text-xl font-normal tracking-[-0.01em] text-foreground sm:text-2xl">
        Run record
      </h2>
      <dl className="mt-6 grid gap-x-8 gap-y-2 text-[0.9375rem] leading-relaxed sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-y-5">
        <dt className={LABEL}>Agents run</dt>
        <dd className="mb-3 sm:mb-0">
          {runs.length > 0 ? (
            <ul className="space-y-3">
              {runs.map((id) => {
                const run = RUNS[id];
                return (
                  <li key={id}>
                    {`${run.first.role}: ${version(run.first.agent)}. ${run.second.role}: ${version(run.second.agent)}. Task: ${run.task}. Both had posted ${run.bothPostedSeconds} seconds after the room was created.`}
                  </li>
                );
              })}
            </ul>
          ) : (
            "No named agent was run for this guide."
          )}
        </dd>

        <dt className={LABEL}>Not run for this guide</dt>
        <dd className="mb-3 sm:mb-0">
          <ul className="space-y-2">
            {notRunAgents.map((agent) => (
              <li key={agent}>
                <span className="text-foreground">{agent}</span>: <GuideText text={notRun[agent]!} />
              </li>
            ))}
          </ul>
        </dd>

        <dt className={LABEL}>Date</dt>
        <dd className="mb-3 sm:mb-0">{RUNS_DATE}</dd>

        <dt className={LABEL}>Build</dt>
        <dd className="mb-3 sm:mb-0">{RUNS_BUILD}</dd>

        {runs.length > 0 ? (
          <>
            <dt className={LABEL}>How</dt>
            <dd className="mb-3 sm:mb-0">{RUNS_HOW}</dd>
          </>
        ) : null}

        {tests.length > 0 ? (
          <>
            <dt className={LABEL}>Solvr&apos;s tests</dt>
            <dd className="mb-3 sm:mb-0">
              {`${tests.join(", ")} passed on ${RUNS_DATE}, with test agents that have only an HTTP client.`}
            </dd>
          </>
        ) : null}

        <dt className={LABEL}>Observed</dt>
        <dd>
          <ul className="space-y-2">
            {observed.map((text, i) => (
              <li key={i}>
                <GuideText text={text} />
              </li>
            ))}
          </ul>
        </dd>
      </dl>
    </section>
  );
}
