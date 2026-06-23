//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/tasklink"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	"github.com/monstercameron/CashFlux/internal/tasktree"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Todo is the budgeting task list: add, complete/reopen, delete.
func Todo() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:tasks", 0)
	bump := func() { rev.Set(rev.Get() + 1) }
	// Re-render after a whole-dataset replacement (undo/redo, import, decrypt) so
	// the list reflects external changes, not just this screen's own mutations.
	_ = uistate.UseDataRevision().Get()

	hideDone := ui.UseState(false)
	// filterPrio is "" (all), "high", "medium", or "low" (C52 lightweight priority
	// filter — lets the user focus on just one priority level without a full filter
	// panel).
	filterPrio := ui.UseState("")
	errMsg := ui.UseState("")

	toggleHideDone := ui.UseEvent(func() { hideDone.Set(!hideDone.Get()) })
	onFilterPrio := ui.UseEvent(func(e ui.Event) { filterPrio.Set(e.GetValue()) })
	// Open the add-task modal from the card header (G6: page-level add affordance,
	// instead of reaching for the global "+" far from the list).
	addTask := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("task") }))

	tasks := app.Tasks()
	accounts := app.Accounts()
	budgets := app.Budgets()
	goals := app.Goals()
	txns := app.Transactions()
	byID := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}

	toggleTask := func(taskID string) {
		t, ok := byID[taskID]
		if !ok {
			return
		}
		if t.Status == domain.StatusDone {
			// Re-opening a done task: plain status flip, no spawn.
			t.Status = domain.StatusOpen
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
		} else {
			// Completing an open task: route through CompleteTask so recurring
			// tasks spawn their next occurrence atomically.
			if err := app.CompleteTask(taskID, id.New(), time.Now()); err != nil {
				errMsg.Set(err.Error())
				return
			}
		}
		bump()
	}
	deleteTask := func(taskID string) {
		// Cascade: deleting a task removes its whole sub-tree (C72).
		for _, d := range tasktree.Descendants(tasks, taskID) {
			_ = app.DeleteTask(d)
		}
		if err := app.DeleteTask(taskID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}
	addSub := func(parentID string) {
		uistate.PromptModal(uistate.T("todo.subtaskPrompt"), "", func(name string) {
			if name == "" {
				return
			}
			t := domain.Task{
				ID: id.New(), Title: name, ParentID: parentID,
				Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceManual,
			}
			if err := app.PutTask(t); err != nil {
				errMsg.Set(err.Error())
				return
			}
			bump()
		})
	}
	saveTask := func(taskID, newTitle, prio, dueStr, newNotes, relType, relID, recur string) {
		t, ok := byID[taskID]
		if !ok {
			return
		}
		if n := strings.TrimSpace(newTitle); n != "" {
			t.Title = n
		}
		if p := domain.TaskPriority(prio); p.Valid() {
			t.Priority = p
		}
		if ds := strings.TrimSpace(dueStr); ds != "" {
			d, err := dateutil.ParseDate(ds)
			if err != nil {
				errMsg.Set(uistate.T("todo.invalidDue"))
				return
			}
			t.Due = d
		} else {
			t.Due = time.Time{}
		}
		t.Notes = strings.TrimSpace(newNotes)
		rt := domain.RelatedType(relType)
		if rt == domain.RelatedNone || rt == "" {
			t.RelatedType = domain.RelatedNone
			t.RelatedID = ""
		} else {
			t.RelatedType = rt
			t.RelatedID = relID
		}
		t.Recurrence = domain.RecurringCadence(recur)
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	// Order + filter, then nest into the parent/child tree (C72). hide-done filters
	// first so a done parent's open child surfaces as a root (tasktree handles it).
	// The priority filter runs after the done filter so it cooperates with hide-done:
	// if you filter to "High" and hide done, only open high-priority tasks appear.
	filtered := tasksort.Visible(tasks, hideDone.Get())
	if p := domain.TaskPriority(filterPrio.Get()); p.Valid() {
		kept := filtered[:0:0]
		for _, t := range filtered {
			if t.Priority == p {
				kept = append(kept, t)
			}
		}
		filtered = kept
	}
	visible := filtered
	nodes := tasktree.Flatten(visible)

	var listBody ui.Node
	switch {
	case len(tasks) == 0:
		listBody = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("todo.empty"), CTALabel: uistate.T("todo.addFirst"), AddTarget: "task"})
	case len(nodes) == 0:
		listBody = P(css.Class("empty"), uistate.T("todo.allDone"))
	default:
		rows := MapKeyed(nodes,
			func(n tasktree.Node) any { return n.Task.ID },
			func(n tasktree.Node) ui.Node {
				return ui.CreateElement(TaskRow, taskRowProps{
					Task: n.Task, Depth: n.Depth,
					OnToggle: toggleTask, OnDelete: deleteTask, OnSave: saveTask, OnAddSub: addSub,
					Accounts: accounts, Budgets: budgets, Goals: goals, Transactions: txns,
				})
			},
		)
		listBody = Div(css.Class("rows"), rows)
	}

	hideLabel := uistate.T("todo.hideDone")
	if hideDone.Get() {
		hideLabel = uistate.T("todo.showAll")
	}

	// Portfolio summary (G6): a compact open/overdue/done count above the list so
	// Nina gets the at-a-glance context every other list screen opens with. Counts
	// are over all tasks, independent of the current view filters.
	openCount, overdueCount, doneCount := 0, 0, 0
	today := dateutil.FormatDate(time.Now())
	for _, t := range tasks {
		if t.Status == domain.StatusDone {
			doneCount++
			continue
		}
		openCount++
		if !t.Due.IsZero() && dateutil.FormatDate(t.Due) < today {
			overdueCount++
		}
	}

	return Section(css.Class("card"),
		Div(css.Class("card-head"),
			H2(css.Class("card-title"), uistate.T("todo.listTitle")),
			Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap, tw.ItemsCenter),
				// Priority filter — lightweight selector; "" means show all (C52).
				Select(css.Class("field"), Attr("aria-label", uistate.T("todo.filterPrioLabel")),
					Attr("data-testid", "todo-filter-prio"), OnChange(onFilterPrio),
					Option(Value(""), SelectedIf(filterPrio.Get() == ""), uistate.T("todo.filterPrioAll")),
					Option(Value(string(domain.PriorityHigh)), SelectedIf(filterPrio.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
					Option(Value(string(domain.PriorityMedium)), SelectedIf(filterPrio.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
					Option(Value(string(domain.PriorityLow)), SelectedIf(filterPrio.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
				),
				Button(css.Class("btn"), Type("button"), OnClick(toggleHideDone), hideLabel),
				Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "todo-add"), Title(uistate.T("todo.addFirst")), OnClick(addTask),
					uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
					Span(uistate.T("todo.addTask"))),
			),
		),
		If(len(tasks) > 0, P(css.Class("todo-summary", tw.TextDim),
			Text(uistate.T("todo.summary", openCount, overdueCount, doneCount)))),
		If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
		listBody,
	)
}

type taskRowProps struct {
	Task         domain.Task
	Depth        int // nesting depth (0 = top level) → indentation (C72)
	OnToggle     func(string)
	OnDelete     func(string)
	OnSave       func(id, title, priority, due, notes, relType, relID, recurrence string)
	OnAddSub     func(parentID string)
	Accounts     []domain.Account
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Transactions []domain.Transaction
}

// TaskRow renders one task with complete/edit/delete. It can be edited inline
// (title, priority, due, notes, entity link). All hooks are declared
// unconditionally so the edit toggle never reorders them.
func TaskRow(props taskRowProps) ui.Node {
	t := props.Task
	dueISO := ""
	if !t.Due.IsZero() {
		dueISO = dateutil.FormatDate(t.Due)
	}

	nav := router.UseNavigate()
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle(t.ID) }))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(t.ID) }))
	addSub := ui.UseEvent(Prevent(func() {
		if props.OnAddSub != nil {
			props.OnAddSub(t.ID)
		}
	}))
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	titleS := ui.UseState(t.Title)
	prioS := ui.UseState(string(t.Priority))
	dueS := ui.UseState(dueISO)
	notesS := ui.UseState(t.Notes)
	// Inline-edit link state — initialised from the task's persisted values.
	editLinkType := ui.UseState(string(t.RelatedType))
	editLinkID := ui.UseState(t.RelatedID)
	editRecur := ui.UseState(string(t.Recurrence))
	onTitle := ui.UseEvent(func(v string) { titleS.Set(v) })
	onPrio := ui.UseEvent(func(e ui.Event) { prioS.Set(e.GetValue()) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })
	onNotes := ui.UseEvent(func(v string) { notesS.Set(v) })
	onEditLinkType := ui.UseEvent(func(e ui.Event) {
		editLinkType.Set(e.GetValue())
		editLinkID.Set("") // reset entity when type changes
	})
	onEditLinkID := ui.UseEvent(func(e ui.Event) { editLinkID.Set(e.GetValue()) })
	onEditRecur := ui.UseEvent(func(e ui.Event) { editRecur.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		titleS.Set(t.Title)
		prioS.Set(string(t.Priority))
		dueS.Set(dueISO)
		notesS.Set(t.Notes)
		editLinkType.Set(string(t.RelatedType))
		editLinkID.Set(t.RelatedID)
		editRecur.Set(string(t.Recurrence))
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(t.ID, titleS.Get(), prioS.Get(), dueS.Get(), notesS.Get(),
			editLinkType.Get(), editLinkID.Get(), editRecur.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("task-edit-" + t.ID)
		}
		return nil
	}, editKey)

	// Row-display deep-link: declared unconditionally here (before any early
	// return) so the hook slot never shifts across renders (framework rule).
	linkRoute := tasklink.Route(t.RelatedType)
	goLink := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath(linkRoute)) }))

	if editing.Get() {
		curEditType := domain.RelatedType(editLinkType.Get())
		var editEntitySelect ui.Node
		if curEditType != domain.RelatedNone && curEditType != "" {
			editEntitySelect = labeledField(uistate.T("todo.linkEntity"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("todo.linkEntity")), OnChange(onEditLinkID),
					buildEntityOptions(curEditType, editLinkID.Get(), props.Accounts, props.Budgets, props.Goals, props.Transactions)))
		}
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				Input(css.Class("field field-wide"), Attr("id", "task-edit-"+t.ID), Type("text"), Placeholder(uistate.T("todo.taskPlaceholder")), Value(titleS.Get()), OnInput(onTitle)),
				labeledField("Priority",
					Select(css.Class("field"), Attr("aria-label", "Priority"), OnChange(onPrio),
						Option(Value(string(domain.PriorityHigh)), SelectedIf(prioS.Get() == string(domain.PriorityHigh)), uistate.T("priority.high")),
						Option(Value(string(domain.PriorityMedium)), SelectedIf(prioS.Get() == string(domain.PriorityMedium)), uistate.T("priority.medium")),
						Option(Value(string(domain.PriorityLow)), SelectedIf(prioS.Get() == string(domain.PriorityLow)), uistate.T("priority.low")),
					)),
				labeledField("Due date",
					Input(css.Class("field"), Type("date"), Attr("aria-label", "Due date"), Value(dueS.Get()), OnInput(onDue))),
				Input(css.Class("field field-wide"), Type("text"), Placeholder(uistate.T("todo.notesEdit")), Value(notesS.Get()), OnInput(onNotes)),
				labeledField(uistate.T("todo.repeat"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("todo.repeat")), Attr("data-testid", "task-edit-repeat-"+t.ID), OnChange(onEditRecur),
						cadenceOptions(editRecur.Get()))),
				labeledField(uistate.T("todo.linkTo"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("todo.linkTo")), OnChange(onEditLinkType),
						linkTypeOptions(editLinkType.Get()))),
				editEntitySelect,
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	// Build the row deep-link node (goLink + linkRoute already declared above).
	var linkNode ui.Node
	if linkRoute != "" {
		name, ok := tasklink.EntityName(t.RelatedType, t.RelatedID,
			props.Accounts, props.Budgets, props.Goals, props.Transactions)
		if ok {
			linkLabel := "→ " + name
			ariaLabel := uistate.T("todo.linkAriaLabel", name)
			linkNode = Button(css.Class("btn row-meta"), Type("button"),
				Attr("aria-label", ariaLabel), Title(ariaLabel), OnClick(goLink), linkLabel)
		} else if t.RelatedID != "" {
			// Entity was deleted — show a muted note rather than nothing.
			linkNode = Span(css.Class("row-meta text-muted"), uistate.T("todo.linkRemoved"))
		}
	}

	done := t.Status == domain.StatusDone
	rowClass := "row"
	glyph := "☐"
	if done {
		rowClass = "row done"
		glyph = "☑"
	}
	plabel, pclass := priorityMeta(t.Priority)

	// Overdue = an open task whose due date is before today. Flag it with the danger
	// tone plus an explicit "overdue" word (colour + text, not colour alone — B15)
	// so a past-due task is actionable at a glance (C52).
	overdue := !done && !t.Due.IsZero() && dateutil.FormatDate(t.Due) < dateutil.FormatDate(time.Now())
	meta := []ui.Node{Span(ClassStr("badge badge-prio "+pclass), plabel)}
	if !t.Due.IsZero() {
		dueText := uistate.T("todo.due") + " " + pr.FormatDate(t.Due)
		dueCls := "row-meta"
		if overdue {
			dueText += " · overdue"
			dueCls = "row-meta text-down"
		}
		meta = append(meta, Span(ClassStr(dueCls), dueText))
	}
	if t.Notes != "" {
		// Truncate long notes inline and expose the full text in a tooltip so the
		// row stays scannable without cutting off information (C52).
		const maxNoteRune = 80
		noteDisplay := t.Notes
		if len([]rune(noteDisplay)) > maxNoteRune {
			noteDisplay = string([]rune(noteDisplay)[:maxNoteRune]) + "…"
		}
		meta = append(meta, Span(css.Class("row-meta"), Title(t.Notes), noteDisplay))
	}
	if t.Recurrence != "" {
		recurLabel := taskCadenceLabel(t.Recurrence)
		meta = append(meta, Span(ClassStr("row-meta badge badge-recur"), Attr("data-testid", "recur-badge-"+t.ID),
			uistate.T("todo.recurBadge", recurLabel)))
	}
	if linkNode != nil {
		meta = append(meta, linkNode)
	}

	if props.Depth > 0 {
		rowClass += " subtask"
	}
	rowArgs := []any{ClassStr(rowClass), Attr("id", t.ID)}
	if props.Depth > 0 {
		rowArgs = append(rowArgs, Style(map[string]string{"margin-left": strconv.Itoa(props.Depth*22) + "px"}))
	}
	rowArgs = append(rowArgs,
		Button(css.Class("check"), Type("button"), Title(uistate.T("todo.toggle")), OnClick(toggle), glyph),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), t.Title),
			Div(css.Class("task-meta"), meta),
		),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("todo.addSubTitle")), OnClick(addSub), uistate.T("todo.addSub")),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("todo.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("todo.deleteTitle")), Title(uistate.T("todo.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
	return Div(rowArgs...)
}

func priorityMeta(p domain.TaskPriority) (label, class string) {
	switch p {
	case domain.PriorityHigh:
		return uistate.T("priority.high"), "prio-high"
	case domain.PriorityLow:
		return uistate.T("priority.low"), "prio-low"
	default:
		return uistate.T("priority.medium"), "prio-med"
	}
}

// linkTypeOptions builds the <option> list for the "Link to" type selector.
// selectedVal is the currently-selected RelatedType string.
func linkTypeOptions(selectedVal string) []ui.Node {
	none := string(domain.RelatedNone)
	return []ui.Node{
		Option(Value(none), SelectedIf(selectedVal == none || selectedVal == ""), uistate.T("todo.linkNone")),
		Option(Value(string(domain.RelatedAccount)), SelectedIf(selectedVal == string(domain.RelatedAccount)), uistate.T("todo.linkAccount")),
		Option(Value(string(domain.RelatedBudget)), SelectedIf(selectedVal == string(domain.RelatedBudget)), uistate.T("todo.linkBudget")),
		Option(Value(string(domain.RelatedGoal)), SelectedIf(selectedVal == string(domain.RelatedGoal)), uistate.T("todo.linkGoal")),
		Option(Value(string(domain.RelatedTransaction)), SelectedIf(selectedVal == string(domain.RelatedTransaction)), uistate.T("todo.linkTransaction")),
	}
}

// buildEntityOptions builds the <option> list for the entity sub-selector that
// appears when a non-None RelatedType is chosen. The first option is a blank
// "— Choose —" prompt so users must make an intentional selection.
func buildEntityOptions(
	rt domain.RelatedType,
	selectedID string,
	accounts []domain.Account,
	budgets []domain.Budget,
	goals []domain.Goal,
	txns []domain.Transaction,
) []ui.Node {
	opts := []ui.Node{
		Option(Value(""), SelectedIf(selectedID == ""), uistate.T("todo.linkEntity")),
	}
	switch rt {
	case domain.RelatedAccount:
		for _, a := range accounts {
			opts = append(opts, Option(Value(a.ID), SelectedIf(selectedID == a.ID), a.Name))
		}
	case domain.RelatedBudget:
		for _, b := range budgets {
			opts = append(opts, Option(Value(b.ID), SelectedIf(selectedID == b.ID), b.Name))
		}
	case domain.RelatedGoal:
		for _, g := range goals {
			opts = append(opts, Option(Value(g.ID), SelectedIf(selectedID == g.ID), g.Name))
		}
	case domain.RelatedTransaction:
		for _, tx := range txns {
			label := tx.Payee
			if label == "" {
				label = tx.Desc
			}
			opts = append(opts, Option(Value(tx.ID), SelectedIf(selectedID == tx.ID), label))
		}
	}
	return opts
}

// cadenceOptions builds the <option> list for a recurrence cadence selector.
// selectedVal is the current RecurringCadence string (empty = no repeat).
func cadenceOptions(selectedVal string) []ui.Node {
	none := ""
	return []ui.Node{
		Option(Value(none), SelectedIf(selectedVal == none), uistate.T("todo.repeatNone")),
		Option(Value(string(domain.CadenceWeekly)), SelectedIf(selectedVal == string(domain.CadenceWeekly)), uistate.T("todo.repeatWeekly")),
		Option(Value(string(domain.CadenceMonthly)), SelectedIf(selectedVal == string(domain.CadenceMonthly)), uistate.T("todo.repeatMonthly")),
		Option(Value(string(domain.CadenceQuarterly)), SelectedIf(selectedVal == string(domain.CadenceQuarterly)), uistate.T("todo.repeatQuarterly")),
		Option(Value(string(domain.CadenceYearly)), SelectedIf(selectedVal == string(domain.CadenceYearly)), uistate.T("todo.repeatYearly")),
	}
}

// taskCadenceLabel returns the human-readable label for a RecurringCadence
// value, used in the recurring-task add/edit selects and the row badge.
func taskCadenceLabel(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return uistate.T("todo.repeatWeekly")
	case domain.CadenceMonthly:
		return uistate.T("todo.repeatMonthly")
	case domain.CadenceQuarterly:
		return uistate.T("todo.repeatQuarterly")
	case domain.CadenceYearly:
		return uistate.T("todo.repeatYearly")
	default:
		return string(c)
	}
}
