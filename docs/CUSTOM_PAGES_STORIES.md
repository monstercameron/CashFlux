# Custom Pages & Workflow Engine — User Stories

Ten stories/use cases that exercise the custom-page, widget, artifact, and workflow
features end to end. Each has acceptance criteria; all are covered by the
`internal/appstate` scenario tests (`scenarios_test.go`) and browser screenshots.

## Custom pages & widgets

1. **Money at a glance (KPI dashboard).**
   *As a budgeter, I create a page with KPI widgets for net worth, income, expense,
   and a savings-rate formula, so my key numbers sit together.*
   **Accept:** each KPI evaluates without error; currency KPIs format in the base
   currency; the savings-rate formula `(income - expense) / income * 100` renders.

2. **Recent activity list.**
   *As a user, I add a List widget of recent transactions so I can scan activity.*
   **Accept:** the list binds to the `transactions` source and shows newest-first rows.

3. **Progress chart.**
   *As a saver, I add a Chart widget so I can watch my net-worth trend.*
   **Accept:** a multi-month net-worth series renders as an area chart.

4. **Notes & reminders.**
   *As a planner, I add a Text widget with my goals/reminders.*
   **Accept:** authored text persists and renders; empty text shows a friendly state.

5. **Imported dataset table.**
   *As an analyst, I import a CSV and show it as a Table widget.*
   **Accept:** the CSV parses into columns + rows; the Table widget renders them.

6. **Brand/receipt image.**
   *As a user, I upload an image artifact and display it via an Image widget.*
   **Accept:** the image persists (bytes) and renders from a data URL.

7. **Organize my pages.**
   *As an organizer, I create several pages, reorder them, hide one, and rename one.*
   **Accept:** order persists; hidden pages drop from the rail (restorable); a rename
   re-slugs uniquely and the page is still reachable.

8. **Mixed-size grid.**
   *As a power user, I place widgets of different sizes and they pack without overlap;
   I can delete a widget.*
   **Accept:** `dashlayout.Pack` produces a non-overlapping layout; deleting a widget
   removes it from both the widget list and the layout.

## Workflow engine

9. **Overspend alert (event-driven).**
   *As a cautious spender, when a transaction is added and expenses exceed income, a
   task is created and I'm notified.*
   **Accept:** adding a qualifying transaction fires the enabled `txn-added` workflow;
   a "Review spending" task appears and a run is recorded; a non-qualifying month does
   nothing.

10. **Tidy-up on demand (manual + dry-run).**
    *As a tidy user, I run an "Apply rules" workflow on demand, previewing it first.*
    **Accept:** a dry run reports the planned effect and changes nothing; a real run
    applies rules and records an audit run.

## Edge cases (also tested)

- A KPI with an invalid formula shows an error, not a crash.
- A List with no source shows an empty state.
- A workflow whose condition is false performs no actions.
- Everything above survives an export → import round trip.
