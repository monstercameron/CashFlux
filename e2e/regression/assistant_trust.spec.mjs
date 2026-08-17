// assistant_trust.spec.mjs — the assistant's trust surface, driven in a real
// browser: what an answer was computed from, what approving a change will do,
// which tab does which job, the per-chat spending cap, and the conversation
// management that used to be missing.
//
// The model is intercepted at the network layer and answered with a canned reply,
// so this exercises the real code — request building, the tool loop, the citation
// capture, the approval gate — without spending anything. Everything except OpenAI
// itself is genuine.
import { test, expect, nav } from "./fixtures.mjs";

const KEY_LABEL = /OpenAI API key/i;

/** Put a key in Settings so the assistant offers its full surface rather than the gate. */
async function configureKey(app) {
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "AI" }).first().click();
  const field = app.getByLabel(KEY_LABEL).first();
  await expect(field).toBeVisible();
  await field.fill("sk-test-not-a-real-key");
  await expect(field).toHaveValue("sk-test-not-a-real-key");
}

/**
 * Answer the assistant's Responses call with a scripted sequence of replies.
 *
 * The reply is a real server-sent-event stream, because the assistant asks for one:
 * a mock returning a single JSON object leaves the client waiting for a completed
 * event that never arrives. Each script entry is either {text} for a plain answer
 * or {tool, args} for a tool call, so a tool loop can be driven turn by turn.
 */
async function mockResponses(app, script) {
  const state = { turn: 0, bodies: [] };
  await app.route("**/responses*", async (route) => {
    let body = null;
    try {
      body = JSON.parse(route.request().postData() || "{}");
    } catch {
      body = null;
    }
    state.bodies.push(body);
    const step = script[Math.min(state.turn, script.length - 1)];
    state.turn++;
    const output = step.tool
      ? [{ type: "function_call", call_id: `call_${state.turn}`, name: step.tool, arguments: JSON.stringify(step.args || {}) }]
      : [{ type: "message", role: "assistant", content: [{ type: "output_text", text: step.text }] }];
    const events = [];
    if (!step.tool) {
      // Deliver the answer in fragments, as the provider does, so these tests
      // exercise the streaming path rather than a shape nothing produces.
      for (const piece of String(step.text).match(/[\s\S]{1,12}/g) || []) {
        events.push("data: " + JSON.stringify({ type: "response.output_text.delta", delta: piece }) + "\n\n");
      }
    }
    events.push("data: " + JSON.stringify({
      type: "response.completed",
      response: { status: "completed", output, usage: { input_tokens: 1200, output_tokens: 90 } },
    }) + "\n\n");
    events.push("data: [DONE]\n\n");
    await route.fulfill({
      status: 200,
      headers: { "content-type": "text/event-stream" },
      body: events.join(""),
    });
  });
  return state;
}


/**
 * Open the saved-conversation list. It lives in the notes rail, which is a drawer,
 * inside a collapsed note — both closed by default so the conversation itself
 * leads the page. A test has to walk the same path a person does.
 */
async function openConversations(app) {
  // On a wide viewport the rail is a permanent sidebar and its toggle is hidden;
  // on a narrow one the toggle opens it as a drawer. Follow whichever path this
  // viewport actually offers rather than assuming one.
  const toggle = app.getByTestId("assistant-aside-toggle");
  if (await toggle.isVisible()) {
    await toggle.click();
  }
  const note = app.getByTestId("assistant-note-convs");
  await expect(note).toBeVisible();
  if ((await note.getAttribute("aria-expanded")) !== "true") {
    await note.click();
  }
  await expect(app.getByTestId("assistant-convs")).toBeVisible();
}

/** Ask the assistant a question and wait for the thread to settle. */
async function ask(app, question) {
  const box = app.locator("#cf-chat-input");
  await expect(box).toBeVisible();
  await box.fill(question);
  await app.getByTestId("assistant-send").click();
}

test.describe("assistant: the three tabs each state their own job", () => {
  test("every tab says what it is for", async ({ app }) => {
    await nav(app, "/assistant");
    const job = app.getByTestId("assistant-tab-job");
    await expect(job).toBeVisible();
    await expect(job).toContainText(/you have a question/i);

    const tabs = app.getByTestId("assistant-hub").locator("button", { hasText: /^Insights$/ });
    await tabs.first().click();
    await expect(job).toContainText(/noticed/i);

    await app.getByTestId("assistant-hub").locator("button", { hasText: /^Automations$/ }).first().click();
    await expect(job).toContainText(/switched on/i);
  });

  test("/insights lands on the briefing, not on a second copy of the chat", async ({ app }) => {
    // The route named after analysis used to render the chat — the same component
    // /assistant renders — so the two URLs were one page under two names.
    await nav(app, "/insights");
    await expect(app.getByTestId("assistant-tab-job")).toContainText(/noticed/i);
    await expect(app.getByTestId("ast-hero-value")).toBeVisible();
  });

  test("/smart lands on Automations", async ({ app }) => {
    await nav(app, "/smart");
    await expect(app.getByTestId("assistant-tab-job")).toContainText(/switched on/i);
  });
});

test.describe("assistant: the Insights briefing covers a chosen period", () => {
  test("changing the period changes the window the hero describes", async ({ app }) => {
    await nav(app, "/insights");
    const range = app.getByTestId("ast-hero-range");
    await expect(range).toBeVisible();
    const monthToDate = await range.innerText();

    await app.getByTestId("assistant-period").selectOption("last-12-months");
    await expect(range).not.toHaveText(monthToDate);
    // A year-long window must actually start a year back, not merely relabel.
    await expect(range).toContainText(/2025/);
  });
});

test.describe("assistant: an answer shows what it was computed from", () => {
  test("a numeric answer carries its sources, with the tool's own result inside", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [
      { tool: "spending_by_category", args: { month: "2026-07" } },
      { text: "You spent $312.40 on groceries in July." },
    ]);
    await nav(app, "/assistant");
    await ask(app, "Explain our grocery spending in July and whether it is unusual");

    const cites = app.getByTestId("assistant-citations");
    await expect(cites).toBeVisible();
    await expect(cites).toContainText(/How I got this/i);
    // Collapsed by default: the point is that the evidence is THERE, not in the way.
    await expect(app.getByTestId("assistant-citations").locator("ul")).toBeHidden();
    await cites.locator("summary").click();
    await expect(cites).toContainText(/Spending by category/);
    await expect(cites).toContainText(/2026-07/);
  });

  test("an answer with no figures in it carries no citation panel", async ({ app }) => {
    // Nothing to check means nothing to show; an empty panel would imply sourcing
    // the answer does not have.
    await configureKey(app);
    await mockResponses(app, [{ text: "Try setting one up under Budgets." }]);
    await nav(app, "/assistant");
    await ask(app, "Give me some general advice about getting organised");
    await expect(app.getByTestId("assistant-citations")).toHaveCount(0);
  });
});

test.describe("assistant: approving a change says what it will do", () => {
  test("the approval card states the change, the reads, and whether it can be undone", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [
      { tool: "add_task", args: { title: "Call the insurer", priority: "high" } },
      { text: "Added it." },
    ]);
    await nav(app, "/assistant");
    await ask(app, "Remind me to call the insurer");

    const card = app.getByTestId("assistant-approval");
    await expect(card).toBeVisible();
    // The tool's own sentence leads.
    await expect(card).toContainText(/Call the insurer/);
    // The structured reading follows: what changes, what is read, and the undo.
    const effects = app.getByTestId("assistant-approval-effects");
    await expect(effects).toContainText(/Adds 1 to-do/);
    await expect(effects).toContainText(/Reads/);
    await expect(app.getByTestId("assistant-approval-undo")).toContainText(/undo this from Activity/i);

    await app.getByTestId("assistant-decline").click();
    await expect(app.getByTestId("assistant-approval")).toHaveCount(0);
  });

  test("a change that cannot be undone says so instead of promising Activity", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [
      { tool: "delete_transaction", args: { id: "t-1" } },
      { text: "Removed." },
    ]);
    await nav(app, "/assistant");
    await ask(app, "Delete that duplicate");
    await expect(app.getByTestId("assistant-approval-undo")).toContainText(/can't be undone/i);
    await app.getByTestId("assistant-decline").click();
  });
});

test.describe("assistant: conversation management", () => {
  test("a chat can be renamed, and clearing the name does not leave it nameless", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [{ text: "You spent $40." }]);
    await nav(app, "/assistant");
    await ask(app, "Tell me about our coffee habit and whether it is worth changing");
    await openConversations(app);

    await app.getByTestId("conv-rename").first().click();
    const input = app.getByTestId("conv-rename-input");
    await input.fill("Coffee habit");
    await input.press("Enter");
    await expect(app.getByTestId("assistant-convs")).toContainText("Coffee habit");

    // Clearing the name re-derives it rather than leaving an empty pill.
    await app.getByTestId("conv-rename").first().click();
    await app.getByTestId("conv-rename-input").fill("");
    await app.getByTestId("conv-rename-input").press("Enter");
    await expect(app.getByTestId("assistant-convs")).not.toContainText("Untitled chat");
  });

  test("searching finds a chat by something said inside it, and says so", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [{ text: "You spent $40 on coffee." }]);
    await nav(app, "/assistant");
    await ask(app, "Tell me about our coffee habit and whether it is worth changing");
    await openConversations(app);

    const search = app.getByTestId("assistant-conv-search");
    await search.fill("coffee");
    // The matched line is shown, so a result says WHY it is a result.
    await expect(app.getByTestId("conv-excerpt").first()).toContainText(/coffee/i);

    await search.fill("something nobody said");
    await expect(app.getByTestId("assistant-convs-empty")).toBeVisible();
  });

  test("an answer can be rated, and the rating stays visible once given", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [{ text: "You spent $40." }]);
    await nav(app, "/assistant");
    await ask(app, "Summarise how this year has gone for us overall");

    const up = app.getByTestId("assistant-rate-up").first();
    await up.click();
    await expect(up).toHaveAttribute("aria-pressed", "true");
    // Visible without hovering: a rating you cannot see is one you cannot check.
    await app.locator("#cf-chat-input").hover();
    await expect(up).toBeVisible();
    // Clicking again clears it — a rating that cannot be taken back stops being given.
    await up.click();
    await expect(up).toHaveAttribute("aria-pressed", "false");
  });
});

test.describe("assistant: the per-chat spending cap", () => {
  test("the cap and the running spend are both offered at the composer", async ({ app }) => {
    await configureKey(app);
    await nav(app, "/assistant");
    // The controls live behind the Chat settings drawer, closed by default so the
    // conversation leads.
    await app.getByTestId("assistant-settings-toggle").click();
    await expect(app.getByTestId("assistant-budget")).toBeVisible();
    await expect(app.getByTestId("assistant-budget-readout")).toBeVisible();
    // The model that will answer is named where the question gets typed.
    await expect(app.getByTestId("assistant-active-model")).toBeVisible();
  });
});

test.describe("assistant: the key gate answers what it costs", () => {
  test("without a key, the gate states cost, source and privacy", async ({ app }) => {
    // No key configured in this test's fresh context.
    await nav(app, "/assistant");
    const gate = app.getByTestId("assistant-key-callout");
    await expect(gate).toBeVisible();
    await expect(gate).toContainText(/a question/i);
    await expect(gate).toContainText(/stay on this device/i);
    await expect(app.getByTestId("assistant-key-link")).toHaveAttribute("href", /platform\.openai\.com/);
  });

  test("without a key the system-prompt editor is not offered", async ({ app }) => {
    // A setting that cannot take effect reads as a broken feature.
    await nav(app, "/assistant");
    await expect(app.getByTestId("assistant-edit-prompt")).toHaveCount(0);
  });
});

test.describe("assistant: the answer stays on screen", () => {
  // The answer is Markdown written imperatively into a node the vdom owns, so the
  // vdom strips it on any re-render of the bubble. Every assertion in this file
  // reads the answer the moment it lands, which is precisely the window in which
  // that bug is invisible: the text appeared and then vanished a beat later, when
  // the session tally, the spend meter or a rating re-rendered the thread.
  //
  // These cases read the answer AFTER something else has re-rendered it.
  const ANSWER = "You spent $312.40 on groceries in August, which is up on July.";

  /** The rendered answer body of the most recent assistant turn. */
  function answerBody(app) {
    return app.locator(".chat-agent-body").last();
  }

  test("the answer survives the re-renders that follow it", async ({ app }) => {
    await configureKey(app);
    await mockResponses(app, [{ text: ANSWER }]);
    await nav(app, "/assistant");
    await ask(app, "How much did we spend on groceries and is that unusual");

    const body = answerBody(app);
    await expect(body).toContainText("$312.40");
    // Long enough for the tally bump, the spend write and the usage note to land.
    await app.waitForTimeout(1500);
    await expect(body).toContainText("$312.40");
  });

  test("rating an answer does not erase it", async ({ app }) => {
    // A rating changes the action row's classes and the turn's stored feedback,
    // re-rendering the bubble without changing a character of the answer.
    await configureKey(app);
    await mockResponses(app, [{ text: ANSWER }]);
    await nav(app, "/assistant");
    await ask(app, "How much did we spend on groceries and is that unusual");

    const body = answerBody(app);
    await expect(body).toContainText("$312.40");
    await app.getByTestId("assistant-rate-up").last().click();
    await expect(app.getByTestId("assistant-rate-up").last()).toHaveAttribute("aria-pressed", "true");
    await expect(body).toContainText("$312.40");
  });

  test("answers already in the thread render on load", async ({ app }) => {
    // The saved conversation's answers mount together on first paint, which is the
    // race the retry was added for — and they were arriving blank.
    await nav(app, "/assistant");
    const bodies = app.locator(".chat-agent-body");
    await expect(bodies.first()).not.toBeEmpty();
  });
});
