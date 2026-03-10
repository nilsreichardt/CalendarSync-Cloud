"use client";

import { useMemo, useState } from "react";

type Connection = {
  id: string;
  email: string;
  displayName?: string;
};

type Calendar = {
  connectionId: string;
  calendarId: string;
  summary: string;
  isPrimary?: boolean;
};

type RuleFormProps = {
  connections: Connection[];
  calendars: Calendar[];
  action: (formData: FormData) => void | Promise<void>;
  initialRule?: ExistingRule;
  title?: string;
  submitLabel?: string;
  cancelHref?: string;
};

type ExistingRule = {
  id: string;
  name: string;
  sourceConnectionId: string;
  sourceCalendarId: string;
  targetConnectionId: string;
  targetCalendarId: string;
  payloadMode: string;
  schedule: string;
  dryRun: boolean;
  updateConcurrency: number;
  start: {
    identifier: string;
    offset: number;
  };
  end: {
    identifier: string;
    offset: number;
  };
  filters?: Array<{
    name: string;
    config?: Record<string, unknown>;
  }>;
  transformations?: Array<{
    name: string;
    config?: Record<string, unknown>;
  }>;
};

type PreviewEvent = {
  title: string;
  day: string;
  start: string;
  end: string;
  location: string;
  description: string;
  meetingLink: string;
  attendees: string;
  reminders: string;
  status: "confirmed" | "declined";
  allDay: boolean;
};

type TransformationOption = {
  name: string;
  help: string;
  configLabel?: string;
  configName?: string;
  configType?: "checkbox" | "text";
  placeholder?: string;
};

type FilterOption = {
  name: string;
  help: string;
  configNames?: string[];
  configLabels?: string[];
  defaults?: string[];
};

const identifiers = ["Now", "TodayStart", "TodayEnd", "MonthStart", "MonthEnd", "YearEnd"] as const;

const baseFieldHelp = {
  name: "A private label for this sync rule. Use something that explains the source, target, and purpose.",
  schedule: "How often the sync job runs. The default runs every 10 minutes. Minimum interval is 10 minutes.",
  sourceConnectionId: "The connected account CalendarSync reads events from.",
  sourceCalendarId: "The calendar inside the source account that provides the original events.",
  targetConnectionId: "The connected account CalendarSync writes synced events into.",
  targetCalendarId: "The destination calendar that receives the transformed copy.",
  payloadMode: "Full details copies the event content. Busy-only creates placeholders and hides most details.",
  startIdentifier: "The start of the sync window. This determines how far back CalendarSync looks for events.",
  startOffset: "Moves the start window backward or forward relative to the selected anchor.",
  endIdentifier: "The end of the sync window. This determines how far into the future CalendarSync looks.",
  endOffset: "Moves the end window backward or forward relative to the selected anchor.",
  updateConcurrency: "How many event updates may run in parallel. Higher values are faster but create more API traffic.",
  dryRun: "Validates and previews the sync without creating or changing events in the target calendar."
} as const;

const transformations: TransformationOption[] = [
  {
    name: "KeepAttendees",
    help: "Copies attendee names or addresses to the target event.",
    configLabel: "Use email as display name",
    configName: "UseEmailAsDisplayName",
    configType: "checkbox"
  },
  { name: "KeepLocation", help: "Copies the event location." },
  { name: "KeepReminders", help: "Copies reminder settings." },
  { name: "KeepDescription", help: "Copies the event description." },
  { name: "KeepMeetingLink", help: "Adds the source meeting URL to the target description." },
  { name: "AddOriginalLink", help: "Adds a link back to the original calendar event." },
  { name: "KeepTitle", help: "Uses the source event title instead of the default CalendarSync title." },
  {
    name: "PrefixTitle",
    help: "Adds a prefix before the target event title.",
    configLabel: "Prefix",
    configName: "Prefix",
    configType: "text",
    placeholder: "[Sync] "
  },
  {
    name: "ReplaceTitle",
    help: "Replaces the target title with a fixed value.",
    configLabel: "Replacement title",
    configName: "NewTitle",
    configType: "text",
    placeholder: "CalendarSync Event"
  }
] as const;

const filters: FilterOption[] = [
  { name: "DeclinedEvents", help: "Skips events you declined." },
  { name: "AllDayEvents", help: "Skips all-day events." },
  {
    name: "TimeFrame",
    help: "Keeps only events inside a daily hour range.",
    configNames: ["HourStart", "HourEnd"],
    configLabels: ["Start hour", "End hour"],
    defaults: ["8", "17"]
  },
  {
    name: "TimeFilter",
    help: "Excludes events inside a daily hour range.",
    configNames: ["HourStart", "HourEnd"],
    configLabels: ["Start hour", "End hour"],
    defaults: ["12", "13"]
  },
  {
    name: "RegexTitle",
    help: "Skips events whose title matches a regular expression.",
    configNames: ["ExcludeRegexp"],
    configLabels: ["Regex"],
    defaults: [".*test"]
  }
] as const;

const defaultPreviewEvent: PreviewEvent = {
  title: "Team sync",
  day: "2026-03-09",
  start: "09:30",
  end: "10:15",
  location: "Room Atlas / Zoom",
  description: "Weekly planning, blockers, and handoffs.",
  meetingLink: "https://meet.google.com/example-sync",
  attendees: "alex@example.com, pat@example.com",
  reminders: "10 minutes before",
  status: "confirmed",
  allDay: false
};

const defaultTransformationSelection: Record<string, boolean> = {
  KeepTitle: true,
  KeepDescription: true
};

const defaultFilterSelection: Record<string, boolean> = {
  DeclinedEvents: true
};

const defaultTransformationConfig: Record<string, string | boolean> = {
  "PrefixTitle.Prefix": "[Sync] ",
  "ReplaceTitle.NewTitle": "CalendarSync Event",
  "KeepAttendees.UseEmailAsDisplayName": false
};

const defaultFilterConfig: Record<string, string> = {
  "TimeFrame.HourStart": "8",
  "TimeFrame.HourEnd": "17",
  "TimeFilter.HourStart": "12",
  "TimeFilter.HourEnd": "13",
  "RegexTitle.ExcludeRegexp": ".*test"
};

function InfoTip({ text }: { text: string }) {
  return (
    <details className="info-tip">
      <summary aria-label="Field information">i</summary>
      <div className="info-tip__body">{text}</div>
    </details>
  );
}

function readHour(value: string) {
  const [hour] = value.split(":");
  const parsed = Number(hour);
  return Number.isFinite(parsed) ? parsed : 0;
}

function matchesRegex(pattern: string, value: string) {
  try {
    return new RegExp(pattern).test(value);
  } catch {
    return false;
  }
}

function buildTransformationsJson(selected: Record<string, boolean>, config: Record<string, string | boolean>) {
  return JSON.stringify(
    transformations
      .filter((item) => selected[item.name])
      .map((item) => {
        const nextConfig: Record<string, unknown> = {};
        if (item.configName) {
          const raw = config[`${item.name}.${item.configName}`];
          if (item.configType === "checkbox") {
            nextConfig[item.configName] = raw === true;
          } else if (typeof raw === "string" && raw.trim() !== "") {
            nextConfig[item.configName] = raw;
          }
        }
        return { name: item.name, config: nextConfig };
      })
  );
}

function buildFiltersJson(selected: Record<string, boolean>, config: Record<string, string>) {
  return JSON.stringify(
    filters
      .filter((item) => selected[item.name])
      .map((item) => {
        const nextConfig: Record<string, unknown> = {};
        item.configNames?.forEach((configName) => {
          const raw = config[`${item.name}.${configName}`];
          if (raw == null || raw === "") {
            return;
          }
          if (configName === "HourStart" || configName === "HourEnd") {
            nextConfig[configName] = Number(raw);
            return;
          }
          nextConfig[configName] = raw;
        });
        return { name: item.name, config: nextConfig };
      })
  );
}

function calendarLabel(calendar: Calendar | undefined) {
  if (!calendar) {
    return "Choose calendar";
  }
  return calendar.summary || calendar.calendarId;
}

function buildSelectedNames(items: ExistingRule["filters"] | ExistingRule["transformations"], defaults: Record<string, boolean>) {
  if (items == null) {
    return { ...defaults };
  }
  if (items.length === 0) {
    return {};
  }

  return items.reduce<Record<string, boolean>>((result, item) => {
    result[item.name] = true;
    return result;
  }, {});
}

function buildConfigMap(
  items: ExistingRule["filters"] | ExistingRule["transformations"],
  defaults: Record<string, string | boolean>
) {
  if (!items?.length) {
    return defaults;
  }

  const nextConfig = { ...defaults };
  for (const item of items) {
    for (const [key, value] of Object.entries(item.config ?? {})) {
      nextConfig[`${item.name}.${key}`] = typeof value === "boolean" ? value : String(value);
    }
  }
  return nextConfig;
}

export function CreateRuleForm({
  connections,
  calendars,
  action,
  initialRule,
  title = "Create Rule",
  submitLabel = "Create rule",
  cancelHref
}: RuleFormProps) {
  const [sourceConnectionId, setSourceConnectionId] = useState(initialRule?.sourceConnectionId ?? "");
  const [targetConnectionId, setTargetConnectionId] = useState(initialRule?.targetConnectionId ?? "");
  const [sourceCalendarId, setSourceCalendarId] = useState(initialRule?.sourceCalendarId ?? "");
  const [targetCalendarId, setTargetCalendarId] = useState(initialRule?.targetCalendarId ?? "");
  const [payloadMode, setPayloadMode] = useState(initialRule?.payloadMode ?? "full");
  const [previewEvent, setPreviewEvent] = useState(defaultPreviewEvent);
  const [selectedTransformations, setSelectedTransformations] = useState<Record<string, boolean>>(
    buildSelectedNames(initialRule?.transformations, defaultTransformationSelection)
  );
  const [selectedFilters, setSelectedFilters] = useState<Record<string, boolean>>(
    buildSelectedNames(initialRule?.filters, defaultFilterSelection)
  );
  const [transformationConfig, setTransformationConfig] = useState<Record<string, string | boolean>>(
    buildConfigMap(initialRule?.transformations, defaultTransformationConfig)
  );
  const [filterConfig, setFilterConfig] = useState<Record<string, string>>(
    buildConfigMap(initialRule?.filters, defaultFilterConfig) as Record<string, string>
  );

  const sourceCalendars = useMemo(
    () => calendars.filter((calendar) => !sourceConnectionId || calendar.connectionId === sourceConnectionId),
    [calendars, sourceConnectionId]
  );
  const targetCalendars = useMemo(
    () => calendars.filter((calendar) => !targetConnectionId || calendar.connectionId === targetConnectionId),
    [calendars, targetConnectionId]
  );

  const sourceCalendar = sourceCalendars.find((calendar) => calendar.calendarId === sourceCalendarId);
  const targetCalendar = targetCalendars.find((calendar) => calendar.calendarId === targetCalendarId);

  const transformationsJson = useMemo(
    () => buildTransformationsJson(selectedTransformations, transformationConfig),
    [selectedTransformations, transformationConfig]
  );
  const filtersJson = useMemo(() => buildFiltersJson(selectedFilters, filterConfig), [selectedFilters, filterConfig]);

  const preview = useMemo(() => {
    const warnings: string[] = [];
    const reasons: string[] = [];

    if (selectedTransformations.ReplaceTitle && (selectedTransformations.KeepTitle || selectedTransformations.PrefixTitle)) {
      warnings.push("ReplaceTitle overrides KeepTitle and PrefixTitle in the current rule order.");
    }

    const eventHour = readHour(previewEvent.start);
    if (selectedFilters.DeclinedEvents && previewEvent.status === "declined") {
      reasons.push("Filtered out because the sample event is declined.");
    }
    if (selectedFilters.AllDayEvents && previewEvent.allDay) {
      reasons.push("Filtered out because the sample event is all-day.");
    }
    if (selectedFilters.TimeFrame) {
      const start = Number(filterConfig["TimeFrame.HourStart"] || "0");
      const end = Number(filterConfig["TimeFrame.HourEnd"] || "24");
      if (eventHour < start || eventHour >= end) {
        reasons.push(`Filtered out because ${previewEvent.start} falls outside ${start}:00-${end}:00.`);
      }
    }
    if (selectedFilters.TimeFilter) {
      const start = Number(filterConfig["TimeFilter.HourStart"] || "0");
      const end = Number(filterConfig["TimeFilter.HourEnd"] || "24");
      if (eventHour >= start && eventHour < end) {
        reasons.push(`Filtered out because ${previewEvent.start} falls inside the blocked range ${start}:00-${end}:00.`);
      }
    }
    if (selectedFilters.RegexTitle) {
      const pattern = filterConfig["RegexTitle.ExcludeRegexp"] || "";
      if (pattern && matchesRegex(pattern, previewEvent.title)) {
        reasons.push(`Filtered out because the title matches ${pattern}.`);
      }
    }

    let title = payloadMode === "busy" ? "Busy" : "CalendarSync Event";
    if (payloadMode === "full" && selectedTransformations.KeepTitle) {
      title = previewEvent.title;
    }
    if (payloadMode === "full" && selectedTransformations.PrefixTitle) {
      title = `${String(transformationConfig["PrefixTitle.Prefix"] || "")}${title}`;
    }
    if (payloadMode === "full" && selectedTransformations.ReplaceTitle) {
      title = String(transformationConfig["ReplaceTitle.NewTitle"] || "CalendarSync Event");
    }

    const descriptionParts: string[] = [];
    if (payloadMode === "full" && selectedTransformations.KeepDescription && previewEvent.description) {
      descriptionParts.push(previewEvent.description);
    }
    if (payloadMode === "full" && selectedTransformations.KeepMeetingLink && previewEvent.meetingLink) {
      descriptionParts.push(`Meeting link: ${previewEvent.meetingLink}`);
    }
    if (payloadMode === "full" && selectedTransformations.AddOriginalLink) {
      descriptionParts.push("Original event: https://calendar.google.com/event?eid=sample");
    }
    if (payloadMode === "busy") {
      descriptionParts.push("Busy-only payload hides description, attendees, and most metadata.");
    }

    return {
      filtered: reasons.length > 0,
      reasons,
      warnings,
      title,
      location: payloadMode === "full" && selectedTransformations.KeepLocation ? previewEvent.location : "",
      attendees: payloadMode === "full" && selectedTransformations.KeepAttendees ? previewEvent.attendees : "",
      reminders: payloadMode === "full" && selectedTransformations.KeepReminders ? previewEvent.reminders : "",
      description: descriptionParts.join("\n\n")
    };
  }, [filterConfig, payloadMode, previewEvent, selectedFilters, selectedTransformations, transformationConfig]);

  return (
    <form action={action} className="rule-builder">
      <input type="hidden" name="transformationsJson" value={transformationsJson} />
      <input type="hidden" name="filtersJson" value={filtersJson} />

      <div className="rule-builder__form">
        <div className="rule-builder__intro">
          <div>
            <h3 style={{ margin: 0 }}>{title}</h3>
            <p className="muted" style={{ marginBottom: 0 }}>
              Build the sync on the left and test it against a sample source event on the right.
            </p>
          </div>
        </div>

        <div className="rule-section">
          <div className="rule-section__header">
            <h4 style={{ margin: 0 }}>Basics</h4>
          </div>
          <div className="rule-fields">
            <label className="field">
              <span className="field__label">
                Rule name
                <InfoTip text={baseFieldHelp.name} />
              </span>
              <input name="name" placeholder="Personal -> Work busy sync" required defaultValue={initialRule?.name ?? ""} />
            </label>

            <label className="field">
              <span className="field__label">
                Schedule
                <InfoTip text={baseFieldHelp.schedule} />
              </span>
              <input name="schedule" defaultValue={initialRule?.schedule ?? "FREQ=MINUTELY;INTERVAL=10"} />
            </label>

            <label className="field">
              <span className="field__label">
                Source connection
                <InfoTip text={baseFieldHelp.sourceConnectionId} />
              </span>
              <select
                name="sourceConnectionId"
                required
                value={sourceConnectionId}
                onChange={(event) => {
                  setSourceConnectionId(event.target.value);
                  setSourceCalendarId("");
                }}
              >
                <option value="">Choose source account</option>
                {connections.map((connection) => (
                  <option key={connection.id} value={connection.id}>
                    {connection.email}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                Source calendar
                <InfoTip text={baseFieldHelp.sourceCalendarId} />
              </span>
              <select name="sourceCalendarId" required value={sourceCalendarId} onChange={(event) => setSourceCalendarId(event.target.value)}>
                <option value="">Choose source calendar</option>
                {sourceCalendars.map((calendar) => (
                  <option key={`src-${calendar.connectionId}-${calendar.calendarId}`} value={calendar.calendarId}>
                    {calendar.summary || calendar.calendarId}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                Target connection
                <InfoTip text={baseFieldHelp.targetConnectionId} />
              </span>
              <select
                name="targetConnectionId"
                required
                value={targetConnectionId}
                onChange={(event) => {
                  setTargetConnectionId(event.target.value);
                  setTargetCalendarId("");
                }}
              >
                <option value="">Choose target account</option>
                {connections.map((connection) => (
                  <option key={connection.id} value={connection.id}>
                    {connection.email}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                Target calendar
                <InfoTip text={baseFieldHelp.targetCalendarId} />
              </span>
              <select name="targetCalendarId" required value={targetCalendarId} onChange={(event) => setTargetCalendarId(event.target.value)}>
                <option value="">Choose target calendar</option>
                {targetCalendars.map((calendar) => (
                  <option key={`dst-${calendar.connectionId}-${calendar.calendarId}`} value={calendar.calendarId}>
                    {calendar.summary || calendar.calendarId}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                Payload mode
                <InfoTip text={baseFieldHelp.payloadMode} />
              </span>
              <select name="payloadMode" value={payloadMode} onChange={(event) => setPayloadMode(event.target.value)}>
                <option value="full">Full details</option>
                <option value="busy">Busy-only placeholder</option>
              </select>
            </label>
          </div>
        </div>

        <div className="rule-section">
          <div className="rule-section__header">
            <h4 style={{ margin: 0 }}>Sync window</h4>
          </div>
          <div className="rule-fields">
            <label className="field">
              <span className="field__label">
                Start anchor
                <InfoTip text={baseFieldHelp.startIdentifier} />
              </span>
              <select name="startIdentifier" defaultValue={initialRule?.start.identifier ?? "TodayStart"}>
                {identifiers.map((identifier) => (
                  <option key={identifier} value={identifier}>
                    {identifier}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                Start offset
                <InfoTip text={baseFieldHelp.startOffset} />
              </span>
              <input name="startOffset" type="number" defaultValue={initialRule?.start.offset ?? 0} />
            </label>

            <label className="field">
              <span className="field__label">
                End anchor
                <InfoTip text={baseFieldHelp.endIdentifier} />
              </span>
              <select name="endIdentifier" defaultValue={initialRule?.end.identifier ?? "YearEnd"}>
                {identifiers.map((identifier) => (
                  <option key={identifier} value={identifier}>
                    {identifier}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field__label">
                End offset
                <InfoTip text={baseFieldHelp.endOffset} />
              </span>
              <input name="endOffset" type="number" defaultValue={initialRule?.end.offset ?? 2} />
            </label>

            <label className="field">
              <span className="field__label">
                Update concurrency
                <InfoTip text={baseFieldHelp.updateConcurrency} />
              </span>
              <input name="updateConcurrency" type="number" min={1} defaultValue={initialRule?.updateConcurrency ?? 1} />
            </label>

            <label className="field field--checkbox">
              <span className="field__label">
                Dry run
                <InfoTip text={baseFieldHelp.dryRun} />
              </span>
              <span className="checkbox-line">
                <input type="checkbox" name="dryRun" defaultChecked={initialRule?.dryRun ?? false} />
                Create no target events
              </span>
            </label>
          </div>
        </div>

        <div className="rule-two-col">
          <section className="rule-section">
            <div className="rule-section__header">
              <h4 style={{ margin: 0 }}>Transformations</h4>
              <span className="muted">How the target event is changed</span>
            </div>
            <div className="choice-list">
              {transformations.map((item) => (
                <div className="choice-card" key={item.name}>
                  <label className="choice-card__title">
                    <span className="checkbox-line">
                      <input
                        type="checkbox"
                        checked={Boolean(selectedTransformations[item.name])}
                        onChange={(event) =>
                          setSelectedTransformations((current) => ({ ...current, [item.name]: event.target.checked }))
                        }
                      />
                      {item.name}
                    </span>
                    <InfoTip text={item.help} />
                  </label>
                  {item.configName && selectedTransformations[item.name] ? (
                    <div className="choice-card__config">
                      {item.configType === "checkbox" ? (
                        <label className="checkbox-line">
                          <input
                            type="checkbox"
                            checked={Boolean(transformationConfig[`${item.name}.${item.configName}`])}
                            onChange={(event) =>
                              setTransformationConfig((current) => ({
                                ...current,
                                [`${item.name}.${item.configName}`]: event.target.checked
                              }))
                            }
                          />
                          {item.configLabel}
                        </label>
                      ) : (
                        <label className="field">
                          <span className="field__label">{item.configLabel}</span>
                          <input
                            value={String(transformationConfig[`${item.name}.${item.configName}`] || "")}
                            placeholder={item.placeholder}
                            onChange={(event) =>
                              setTransformationConfig((current) => ({
                                ...current,
                                [`${item.name}.${item.configName}`]: event.target.value
                              }))
                            }
                          />
                        </label>
                      )}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </section>

          <section className="rule-section">
            <div className="rule-section__header">
              <h4 style={{ margin: 0 }}>Filters</h4>
              <span className="muted">When CalendarSync should skip an event</span>
            </div>
            <div className="choice-list">
              {filters.map((item) => (
                <div className="choice-card" key={item.name}>
                  <label className="choice-card__title">
                    <span className="checkbox-line">
                      <input
                        type="checkbox"
                        checked={Boolean(selectedFilters[item.name])}
                        onChange={(event) => setSelectedFilters((current) => ({ ...current, [item.name]: event.target.checked }))}
                      />
                      {item.name}
                    </span>
                    <InfoTip text={item.help} />
                  </label>
                  {item.configNames && selectedFilters[item.name] ? (
                    <div className="choice-card__config choice-card__config--grid">
                      {item.configNames.map((configName, index) => (
                        <label className="field" key={configName}>
                          <span className="field__label">{item.configLabels?.[index] || configName}</span>
                          <input
                            value={filterConfig[`${item.name}.${configName}`] || ""}
                            onChange={(event) =>
                              setFilterConfig((current) => ({
                                ...current,
                                [`${item.name}.${configName}`]: event.target.value
                              }))
                            }
                          />
                        </label>
                      ))}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </section>
        </div>

        <div style={{ marginTop: 12 }}>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button className="btn">{submitLabel}</button>
            {cancelHref ? (
              <a className="btn secondary" href={cancelHref}>
                Cancel
              </a>
            ) : null}
          </div>
        </div>
      </div>

      <aside className="rule-builder__preview">
        <section className="rule-section">
          <div className="rule-section__header">
            <h4 style={{ margin: 0 }}>Preview</h4>
            <span className="muted">Edit a sample source event and see the target result live</span>
          </div>

          <div className="preview-stack">
            <div className="preview-card preview-card--source">
              <div className="preview-card__header">
                <strong>Source event</strong>
                <span className="preview-chip">{calendarLabel(sourceCalendar)}</span>
              </div>
              <div className="rule-fields rule-fields--single">
                <label className="field">
                  <span className="field__label">Title</span>
                  <input value={previewEvent.title} onChange={(event) => setPreviewEvent((current) => ({ ...current, title: event.target.value }))} />
                </label>
                <div className="rule-fields">
                  <label className="field">
                    <span className="field__label">Day</span>
                    <input
                      type="date"
                      value={previewEvent.day}
                      onChange={(event) => setPreviewEvent((current) => ({ ...current, day: event.target.value }))}
                    />
                  </label>
                  <label className="field field--checkbox">
                    <span className="field__label">All-day</span>
                    <span className="checkbox-line">
                      <input
                        type="checkbox"
                        checked={previewEvent.allDay}
                        onChange={(event) => setPreviewEvent((current) => ({ ...current, allDay: event.target.checked }))}
                      />
                      Treat as all-day event
                    </span>
                  </label>
                </div>
                {!previewEvent.allDay ? (
                  <div className="rule-fields">
                    <label className="field">
                      <span className="field__label">Start</span>
                      <input
                        type="time"
                        value={previewEvent.start}
                        onChange={(event) => setPreviewEvent((current) => ({ ...current, start: event.target.value }))}
                      />
                    </label>
                    <label className="field">
                      <span className="field__label">End</span>
                      <input
                        type="time"
                        value={previewEvent.end}
                        onChange={(event) => setPreviewEvent((current) => ({ ...current, end: event.target.value }))}
                      />
                    </label>
                  </div>
                ) : null}
                <label className="field">
                  <span className="field__label">Location</span>
                  <input
                    value={previewEvent.location}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, location: event.target.value }))}
                  />
                </label>
                <label className="field">
                  <span className="field__label">Meeting link</span>
                  <input
                    value={previewEvent.meetingLink}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, meetingLink: event.target.value }))}
                  />
                </label>
                <label className="field">
                  <span className="field__label">Attendees</span>
                  <input
                    value={previewEvent.attendees}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, attendees: event.target.value }))}
                  />
                </label>
                <label className="field">
                  <span className="field__label">Reminders</span>
                  <input
                    value={previewEvent.reminders}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, reminders: event.target.value }))}
                  />
                </label>
                <label className="field">
                  <span className="field__label">Description</span>
                  <textarea
                    rows={4}
                    value={previewEvent.description}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, description: event.target.value }))}
                  />
                </label>
                <label className="field">
                  <span className="field__label">Attendance</span>
                  <select
                    value={previewEvent.status}
                    onChange={(event) => setPreviewEvent((current) => ({ ...current, status: event.target.value as PreviewEvent["status"] }))}
                  >
                    <option value="confirmed">Confirmed</option>
                    <option value="declined">Declined</option>
                  </select>
                </label>
              </div>
            </div>

            <div className="preview-arrow" aria-hidden="true">
              →
            </div>

            <div className="preview-card preview-card--target">
              <div className="preview-card__header">
                <strong>Target event</strong>
                <span className="preview-chip">{calendarLabel(targetCalendar)}</span>
              </div>
              {preview.filtered ? (
                <div className="preview-empty">
                  <strong>Event will not sync</strong>
                  <ul className="preview-notes">
                    {preview.reasons.map((reason) => (
                      <li key={reason}>{reason}</li>
                    ))}
                  </ul>
                </div>
              ) : (
                <div className="event-card">
                  <div className="event-card__top">
                    <div>
                      <strong>{preview.title}</strong>
                      <div className="muted">
                        {previewEvent.day}
                        {previewEvent.allDay ? " · All day" : ` · ${previewEvent.start}-${previewEvent.end}`}
                      </div>
                    </div>
                    <span className="event-card__mode">{payloadMode === "busy" ? "Busy-only" : "Full details"}</span>
                  </div>
                  {preview.location ? <div className="event-card__row">Location: {preview.location}</div> : null}
                  {preview.attendees ? <div className="event-card__row">Attendees: {preview.attendees}</div> : null}
                  {preview.reminders ? <div className="event-card__row">Reminders: {preview.reminders}</div> : null}
                  <div className="event-card__description">{preview.description || "No description copied to target event."}</div>
                </div>
              )}

              {preview.warnings.length > 0 ? (
                <div className="preview-warning">
                  {preview.warnings.map((warning) => (
                    <div key={warning}>{warning}</div>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        </section>
      </aside>
    </form>
  );
}
