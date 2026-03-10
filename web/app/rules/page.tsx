import { auth } from "@/auth";
import { apiDelete, apiGet, apiPost } from "@/lib/api";
import { createRuleAction, updateRuleAction } from "./actions";
import { CreateRuleForm } from "./create-rule-form";
import { revalidatePath } from "next/cache";
import Link from "next/link";

type RulesPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function RulesPage({ searchParams }: RulesPageProps) {
  const session = await auth();
  if (!session) {
    return <div className="card">Sign in to configure sync rules.</div>;
  }

  const resolvedSearchParams = searchParams ? await searchParams : {};
  const confirmDeleteId =
    typeof resolvedSearchParams.confirmDelete === "string" ? resolvedSearchParams.confirmDelete : "";
  const editRuleId = typeof resolvedSearchParams.edit === "string" ? resolvedSearchParams.edit : "";

  const [rulesData, connectionsData, calendarsData] = await Promise.all([
    apiGet("/api/rules"),
    apiGet("/api/connections"),
    apiGet("/api/calendars")
  ]);
  const rules = rulesData.rules ?? [];
  const connections = connectionsData.connections ?? [];
  const calendars = calendarsData.calendars ?? [];
  const ruleToConfirm = rules.find((rule: any) => rule.id === confirmDeleteId);
  const ruleToEdit = rules.find((rule: any) => rule.id === editRuleId);

  return (
    <div>
      {ruleToConfirm && (
        <div className="card">
          <h3 style={{ marginTop: 0 }}>Delete Rule</h3>
          <p className="muted" style={{ marginTop: 0 }}>
            Choose whether to remove only the rule or disable it first and clean up the synced events in the target calendar.
          </p>
          <div style={{ marginBottom: 12 }}>
            <strong>{ruleToConfirm.name}</strong>
            <div className="muted">
              {ruleToConfirm.sourceCalendarId} {"->"} {ruleToConfirm.targetCalendarId}
            </div>
          </div>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <form
              action={async () => {
                "use server";
                await apiDelete(`/api/rules/${ruleToConfirm.id}`);
                revalidatePath("/rules");
              }}
            >
              <button className="btn secondary">Delete Rule Only</button>
            </form>
            <form
              action={async () => {
                "use server";
                await apiPost(`/api/rules/${ruleToConfirm.id}/cleanup`, {});
                revalidatePath("/runs");
                revalidatePath("/rules");
              }}
            >
              <button className="btn">Disable And Cleanup</button>
            </form>
            <Link className="btn secondary" href="/rules">
              Cancel
            </Link>
          </div>
          <div className="muted" style={{ marginTop: 10 }}>
            Cleanup disables the rule immediately and starts a cleanup run. Delete-only removes the rule immediately.
          </div>
        </div>
      )}

      <div className="card">
        <CreateRuleForm
          key={ruleToEdit?.id ?? "new-rule"}
          action={
            ruleToEdit
              ? async (formData) => {
                  "use server";
                  await updateRuleAction(ruleToEdit.id, formData);
                }
              : createRuleAction
          }
          connections={connections}
          calendars={calendars}
          initialRule={ruleToEdit}
          title={ruleToEdit ? "Edit Rule" : "Create Rule"}
          submitLabel={ruleToEdit ? "Save changes" : "Create rule"}
          cancelHref={ruleToEdit ? "/rules" : undefined}
        />
      </div>

      {rules.map((rule: any) => (
        <div className="card" key={rule.id}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
            <div>
              <strong>{rule.name}</strong>
              <div className="muted">
                {rule.sourceCalendarId} {"->"} {rule.targetCalendarId} ({rule.payloadMode}
                {rule.enabled ? "" : ", disabled"})
              </div>
            </div>
            <div style={{ display: "flex", gap: 8 }}>
              <form
                action={async () => {
                  "use server";
                  await apiPost(`/api/rules/${rule.id}/run`, {});
                  revalidatePath("/runs");
                }}
              >
                <button className="btn">Run</button>
              </form>
              <form
                action={async () => {
                  "use server";
                  await apiPost(`/api/rules/${rule.id}/cleanup`, {});
                  revalidatePath("/runs");
                }}
              >
                <button className="btn secondary">Cleanup</button>
              </form>
              <Link className="btn secondary" href={`/rules?edit=${rule.id}`}>
                Edit
              </Link>
              <Link className="btn danger" href={`/rules?confirmDelete=${rule.id}`}>
                Delete
              </Link>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
