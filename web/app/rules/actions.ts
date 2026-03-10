"use server";

import { apiPatch, apiPost } from "@/lib/api";
import { revalidatePath } from "next/cache";

const transformerNames = new Set([
  "KeepAttendees",
  "KeepLocation",
  "KeepReminders",
  "KeepDescription",
  "KeepMeetingLink",
  "AddOriginalLink",
  "KeepTitle",
  "PrefixTitle",
  "ReplaceTitle"
]);

const filterNames = new Set(["DeclinedEvents", "AllDayEvents", "TimeFrame", "TimeFilter", "RegexTitle"]);

type NamedConfig = {
  name: string;
  config?: Record<string, unknown>;
};

function parseNamedConfig(raw: FormDataEntryValue | null, allowedNames: Set<string>): NamedConfig[] {
  if (typeof raw !== "string" || raw.trim() === "") {
    return [];
  }

  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed.flatMap((item) => {
      if (!item || typeof item !== "object") {
        return [];
      }

      const name = typeof item.name === "string" ? item.name : "";
      if (!allowedNames.has(name)) {
        return [];
      }

      const config = item.config && typeof item.config === "object" ? item.config : {};
      return [{ name, config }];
    });
  } catch {
    return [];
  }
}

function readNumber(raw: FormDataEntryValue | null, fallback: number) {
  const value = typeof raw === "string" ? Number(raw) : NaN;
  return Number.isFinite(value) ? value : fallback;
}

function buildRulePayload(formData: FormData) {
  const transformations = parseNamedConfig(formData.get("transformationsJson"), transformerNames);
  const filters = parseNamedConfig(formData.get("filtersJson"), filterNames);

  return {
    name: formData.get("name"),
    sourceConnectionId: formData.get("sourceConnectionId"),
    sourceCalendarId: formData.get("sourceCalendarId"),
    targetConnectionId: formData.get("targetConnectionId"),
    targetCalendarId: formData.get("targetCalendarId"),
    payloadMode: formData.get("payloadMode"),
    direction: "one_way",
    schedule: formData.get("schedule"),
    enabled: true,
    dryRun: formData.get("dryRun") === "on",
    updateConcurrency: readNumber(formData.get("updateConcurrency"), 1),
    start: {
      identifier: formData.get("startIdentifier"),
      offset: readNumber(formData.get("startOffset"), 0)
    },
    end: {
      identifier: formData.get("endIdentifier"),
      offset: readNumber(formData.get("endOffset"), 2)
    },
    filters,
    transformations
  };
}

export async function createRuleAction(formData: FormData) {
  await apiPost("/api/rules", buildRulePayload(formData));

  revalidatePath("/rules");
}

export async function updateRuleAction(ruleId: string, formData: FormData) {
  await apiPatch(`/api/rules/${ruleId}`, buildRulePayload(formData));

  revalidatePath("/rules");
}
