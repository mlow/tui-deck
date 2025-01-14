package deck_card

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sort"
	"strconv"
	"time"
	"tui-deck/deck_comment"
	"tui-deck/deck_help"
	"tui-deck/deck_http"
	"tui-deck/deck_markdown"
	"tui-deck/deck_stack"
	"tui-deck/deck_structs"
	"tui-deck/deck_ui"
	"tui-deck/utils"
)

var DetailText *tview.TextView
var DetailEditText *tview.TextArea
var EditTagsFlex *tview.Flex
var EditUsersFlex *tview.Flex
var Modal *tview.Modal

var CardsMap = make(map[int]deck_structs.Card)
var EditableCard = deck_structs.Card{}

var currentBoard deck_structs.Board

var app *tview.Application
var configuration utils.Configuration

func Init(application *tview.Application, conf utils.Configuration, board deck_structs.Board) {

	app = application
	configuration = conf

	DetailText = tview.NewTextView()
	DetailEditText = tview.NewTextArea()
	EditTagsFlex = tview.NewFlex()
	EditUsersFlex = tview.NewFlex()

	Modal = tview.NewModal()
	currentBoard = board
}

func SetCurrentBoard(board deck_structs.Board) {
	currentBoard = board
}

func BuildCardViewer() {
	DetailText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// ESC -> back to main view
			deck_ui.BuildFullFlex(deck_ui.MainFlex, nil)
		} else if event.Rune() == 101 {
			// e -> edit description
			DetailEditText.SetTitle(fmt.Sprintf(" %s- EDIT", DetailText.GetTitle()))
			DetailEditText.SetText(utils.FormatDescription(EditableCard.Description), true)
			deck_ui.BuildFullFlex(DetailEditText, nil)

		} else if event.Rune() == 99 {
			// c -> comments
			cardId := utils.GetId(DetailText.GetTitle())
			deck_comment.GetComments(cardId)
			deck_comment.CommentTree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape {
					// ESC -> back to main view
					deck_ui.BuildFullFlex(DetailText, nil)
					return nil
				}
				if event.Key() == tcell.KeyTAB {
					return nil
				}
				if event.Key() == tcell.KeyRight {
					return nil
				}
				if event.Key() == tcell.KeyLeft {
					return nil
				}
				if event.Rune() == 97 {
					// a -> add comment
					addForm, comment := deck_comment.BuildAddForm(deck_structs.Comment{})
					addForm.AddButton("Save", func() {
						err := deck_comment.AddComment(cardId, *comment)
						deck_comment.CreateCommentsTree()
						deck_ui.BuildFullFlex(deck_comment.CommentTree, err)
					})
					deck_ui.BuildFullFlex(addForm, nil)
					return nil
				} else if event.Rune() == 100 {
					// d -> delete comment
					commentId := deck_comment.CommentTree.GetCurrentNode().GetReference().(int)
					deck_comment.DeleteComment(cardId, commentId)
					return nil
				} else if event.Rune() == 114 {
					// r -> reply comment
					parentId := deck_comment.CommentTree.GetCurrentNode().GetReference().(int)
					addForm, comment := deck_comment.BuildAddForm(deck_structs.Comment{})
					addForm.AddButton("Save", func() {
						err := deck_comment.ReplyComment(cardId, parentId, *comment)
						deck_comment.CreateCommentsTree()
						deck_ui.BuildFullFlex(deck_comment.CommentTree, err)
					})
					deck_ui.BuildFullFlex(addForm, nil)
					return nil
				} else if event.Rune() == 101 {
					// e -> edit comment
					commentId := deck_comment.CommentTree.GetCurrentNode().GetReference().(int)
					comment := deck_comment.CommentsMap[commentId]
					editForm, editComment := deck_comment.BuildAddForm(comment)
					editForm.AddButton("Save", func() {
						go func() {
							err := deck_comment.EditComment(cardId, *editComment)
							if err != nil {
								deck_ui.FooterBar.SetText(fmt.Sprintf("Error editing new comment: %s", err.Error()))
							}
						}()
						deck_comment.CreateCommentsTree()
						deck_ui.BuildFullFlex(deck_comment.CommentTree, nil)
					})
					deck_ui.BuildFullFlex(editForm, nil)
					return nil
				} else if event.Rune() == 63 {
					// ? -> help
					deck_ui.BuildHelp(deck_comment.CommentTree, deck_help.HelpComments)
					return nil
				}
				return event
			})

			deck_comment.CreateCommentsTree()

			deck_comment.CommentTree.SetTitle(fmt.Sprintf(" %s- COMMENTS ", DetailText.GetTitle()))
			deck_ui.BuildFullFlex(deck_comment.CommentTree, nil)

		} else if event.Rune() == 108 {
			// l -> labels
			EditTagsFlex.Clear()
			actualLabelList := tview.NewList()
			actualLabelList.SetBorder(true)
			actualLabelList.SetTitle(" delete labels ")
			actualLabelList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyTab {
					return nil
				}
				return event
			})
			for _, label := range EditableCard.Labels {
				actualLabelList.AddItem(fmt.Sprintf("[#%s]%s", label.Color, label.Title), "",
					rune(0), nil)
			}
			actualLabelList.SetSelectedFunc(func(index int, name string, secondName string, rune rune) {
				label := EditableCard.Labels[index]
				jsonBody := fmt.Sprintf(`{"labelId": %d}`, label.Id)
				go DeleteLabel(jsonBody)
				EditableCard.Labels = append(EditableCard.Labels[:index], EditableCard.Labels[index+1:]...)
				CardsMap[EditableCard.Id] = EditableCard
				actualLabelList.RemoveItem(index)

				updateStacks()
				BuildStacks()
				app.SetFocus(actualLabelList)
			})

			labelList := tview.NewList()
			labelList.SetBorder(true)
			labelList.SetTitle(" add labels")
			labelList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyTab {
					return nil
				}
				return event
			})
			for _, label := range currentBoard.Labels {
				labelList.AddItem(fmt.Sprintf("[#%s]%s", label.Color, label.Title), "",
					rune(0), nil)
			}

			labelList.SetSelectedFunc(func(index int, name string, secondName string, rune rune) {
				label := currentBoard.Labels[index]

				for _, l := range EditableCard.Labels {
					if l.Id == label.Id {
						deck_ui.FooterBar.SetText("label already assigned")
						return
					}
				}

				jsonBody := fmt.Sprintf(`{"labelId": %d }`, label.Id)
				go AssignLabel(jsonBody)
				EditableCard.Labels = append(EditableCard.Labels, label)
				CardsMap[EditableCard.Id] = EditableCard
				actualLabelList.AddItem(fmt.Sprintf("[#%s]%s", label.Color, label.Title), "",
					rune, nil)
				updateStacks()
				BuildStacks()
				app.SetFocus(labelList)
			})

			EditTagsFlex.SetDirection(tview.FlexColumn)
			EditTagsFlex.SetBorder(true)
			EditTagsFlex.SetBorderColor(utils.GetColor(configuration.Color))
			EditTagsFlex.SetTitle(fmt.Sprintf(" %s- EDIT TAGS ", DetailText.GetTitle()))

			EditTagsFlex.AddItem(actualLabelList, 0, 1, true)
			EditTagsFlex.AddItem(labelList, 0, 1, true)
			EditTagsFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEsc {
					deck_ui.BuildFullFlex(DetailText, nil)
					return nil
				}
				if event.Key() == tcell.KeyTab {
					focus := app.GetFocus().(*tview.List)
					if focus == actualLabelList {
						app.SetFocus(labelList)
					} else {
						app.SetFocus(actualLabelList)
					}
				} else if event.Rune() == 63 {
					// ? deck_help menu
					deck_ui.BuildHelp(EditTagsFlex, deck_help.HelpLabels)
				}
				return event
			})

			deck_ui.BuildFullFlex(EditTagsFlex, nil)

		} else if event.Rune() == 117 {
			// u -> edit users
			EditUsersFlex.Clear()
			actualUserList := tview.NewList()
			actualUserList.SetBorder(true)
			actualUserList.SetTitle(" delete user ")
			actualUserList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyTab {
					return nil
				}
				return event
			})
			for _, user := range EditableCard.AssignedUsers {
				actualUserList.AddItem(fmt.Sprintf("%s", user.Participant.DisplayName), "",
					rune(0), nil)
			}
			actualUserList.SetSelectedFunc(func(index int, name string, secondName string, rune rune) {
				user := EditableCard.AssignedUsers[index]
				// delete user
				jsonBody := fmt.Sprintf(`{"userId": "%s"}`, user.Participant.Uid)
				go DeleteUser(jsonBody)
				EditableCard.AssignedUsers = append(EditableCard.AssignedUsers[:index], EditableCard.AssignedUsers[index+1:]...)
				CardsMap[EditableCard.Id] = EditableCard
				actualUserList.RemoveItem(index)

				updateStacks()
				BuildStacks()
				app.SetFocus(actualUserList)

			})

			userList := tview.NewList()
			userList.SetBorder(true)
			userList.SetTitle(" add users")
			userList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyTab {
					return nil
				}
				return event
			})
			for _, user := range currentBoard.Users {
				userList.AddItem(fmt.Sprintf("%s", user.DisplayName), "",
					rune(0), nil)
			}

			userList.SetSelectedFunc(func(index int, name string, secondName string, rune rune) {
				user := currentBoard.Users[index]

				for _, u := range EditableCard.AssignedUsers {
					if u.Participant.Uid == user.Uid {
						deck_ui.FooterBar.SetText("user already assigned")
						return
					}
				}

				jsonBody := fmt.Sprintf(`{"userId": "%s" }`, user.Uid)
				go AssignUser(jsonBody)

				au := deck_structs.AssignedUser{
					CardId: EditableCard.Id,
					Type:   0,
					Participant: deck_structs.Owner{
						PrimaryKey:  user.PrimaryKey,
						Uid:         user.Uid,
						DisplayName: user.DisplayName,
					},
				}
				EditableCard.AssignedUsers = append(EditableCard.AssignedUsers, au)
				CardsMap[EditableCard.Id] = EditableCard
				actualUserList.AddItem(fmt.Sprintf("%s", user.DisplayName), "",
					rune, nil)
				updateStacks()
				BuildStacks()
				app.SetFocus(userList)
			})

			EditUsersFlex.SetDirection(tview.FlexColumn)
			EditUsersFlex.SetBorder(true)
			EditUsersFlex.SetBorderColor(utils.GetColor(configuration.Color))
			EditUsersFlex.SetTitle(fmt.Sprintf(" %s- EDIT Users ", DetailText.GetTitle()))

			EditUsersFlex.AddItem(actualUserList, 0, 1, true)
			EditUsersFlex.AddItem(userList, 0, 1, true)
			EditUsersFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEsc {
					deck_ui.BuildFullFlex(DetailText, nil)
					return nil
				}
				if event.Key() == tcell.KeyTab {
					focus := app.GetFocus().(*tview.List)
					if focus == actualUserList {
						app.SetFocus(userList)
					} else {
						app.SetFocus(actualUserList)
					}
				} else if event.Rune() == 63 {
					// ? deck_help menu
					deck_ui.BuildHelp(EditUsersFlex, deck_help.HelpUsers)
				}
				return event
			})

			deck_ui.BuildFullFlex(EditUsersFlex, nil)

		} else if event.Rune() == 116 {
			// t -> edit detail
			var form *tview.Form
			form, card := BuildDetailForm(&EditableCard)
			EditableCard = *card

			form.AddButton("Save", func() {
				go editCard()
				if len(EditableCard.DueDate) > 0 {
					pattern := "02/01/2006 15:04"
					parse, _ := time.Parse(pattern, EditableCard.DueDate)
					EditableCard.DueDate = parse.Format("2006-01-02T15:04:05+00:00")
				}
				CardsMap[EditableCard.Id] = EditableCard
				DetailText.SetTitle(fmt.Sprintf(" %s ", EditableCard.Title))
				updateStacks()
				BuildStacks()
				deck_ui.BuildFullFlex(DetailText, nil)
			})
			deck_ui.BuildFullFlex(form, nil)
		} else if event.Rune() == 63 {
			// ? -> deck_help menu
			deck_ui.BuildHelp(DetailText, deck_help.HelpView)
		}
		return event
	})

	DetailEditText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			DetailText.Clear()
			DetailText.SetTitle(fmt.Sprintf(" %s ", EditableCard.Title))
			DetailText.SetText(deck_markdown.GetMarkDownDescription(utils.FormatDescription(EditableCard.Description), configuration))
			deck_ui.BuildFullFlex(DetailText, nil)
		} else if event.Key() == tcell.KeyF2 {
			EditableCard.Description = DetailEditText.GetText()
			go editCard()
			CardsMap[EditableCard.Id] = EditableCard
			DetailText.SetText(deck_markdown.GetMarkDownDescription(utils.FormatDescription(EditableCard.Description), configuration))
			deck_ui.BuildFullFlex(DetailText, nil)
		}
		return event
	})
	DetailText.SetBorder(true)
	DetailText.SetBorderColor(utils.GetColor(configuration.Color))

	DetailEditText.SetBorder(true)
	DetailEditText.SetBorderColor(utils.GetColor(configuration.Color))
}

func updateStacks() {
	for i, s := range deck_stack.Stacks {
		if s.Id == EditableCard.StackId {
			for j, c := range s.Cards {
				if c.Id == EditableCard.Id {
					deck_stack.Stacks[i].Cards[j] = EditableCard
					break
				}
			}
			break
		}
	}
}

func moveCardToStack(todoList *tview.List, primitive *tview.Primitive, key tcell.Key) {
	i := todoList.GetCurrentItem()
	name, _ := todoList.GetItemText(i)
	cardId := utils.GetId(name)
	card := CardsMap[cardId]

	actualPrimitiveIndex := deck_ui.Primitives[*primitive]

	var operator int

	switch key {
	case tcell.KeyLeft:
		if actualPrimitiveIndex == 0 {
			return
		}
		operator = -1

		break
	case tcell.KeyRight:
		if actualPrimitiveIndex == len(deck_stack.Stacks)-1 {
			return
		}
		operator = 1
		break
	}

	nextStack := deck_stack.Stacks[actualPrimitiveIndex+operator]

	jsonBody := fmt.Sprintf(`{"stackId": "%d", "title": "%s", "type": "plain", "owner":"%s"}`,
		nextStack.Id, utils.CleanText(card.Title), configuration.User)

	go updateCard(currentBoard.Id, card.StackId, card.Id, jsonBody)

	var labels = utils.BuildLabels(card)
	card.StackId = nextStack.Id
	CardsMap[card.Id] = card

	destList := deck_ui.GetNextFocus(actualPrimitiveIndex + operator).(*tview.List)
	todoList.RemoveItem(i)

	dueDate := ""
	if len(card.DueDate) > 0 {
		parse, _ := time.Parse("2006-01-02T15:04:05+00:00", card.DueDate)
		card.DueDate = parse.Format("02/01/2006 15:04")
		dueDate = fmt.Sprintf("- [red:-:-](%s)[white]", card.DueDate)
	}

	assigners := make([]string, 0)
	for _, o := range card.AssignedUsers {
		assigners = append(assigners, o.Participant.GetAbbrv())
	}

	assignersFormatter := ""
	if len(assigners) > 0 {
		assignersFormatter = fmt.Sprintf("- [red:gray:-]%s[-:-:-] ", utils.CommaString(assigners))
	}

	destList.InsertItem(0, fmt.Sprintf("[%s]#%d[white] %s- %s %s", configuration.Color, card.Id, assignersFormatter, card.Title, dueDate), labels, rune(0), nil)
	destList.SetCurrentItem(0)
	app.SetFocus(destList)
}

func BuildAddForm() (*tview.Form, *deck_structs.Card) {
	addForm := tview.NewForm()
	card := deck_structs.Card{}
	addForm.SetTitle(" Add Card ")
	addForm.SetBorder(true)
	addForm.SetBorderColor(utils.GetColor(configuration.Color))
	addForm.SetButtonBackgroundColor(utils.GetColor(configuration.Color))
	addForm.SetFieldBackgroundColor(tcell.ColorWhite)
	addForm.SetFieldTextColor(tcell.ColorBlack)
	addForm.SetLabelColor(utils.GetColor(configuration.Color))
	addForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			deck_ui.BuildFullFlex(deck_ui.MainFlex, nil)
			return nil
		}
		return event
	})
	addForm.AddInputField("Title", "", 20, nil, func(title string) {
		card.Title = title
	})
	addForm.AddTextArea("Description", "", 60, 10, 300, func(description string) {
		card.Description = description
	})

	addForm.AddInputField("Due Date", "", 18, func(textToCheck string, lastChar rune) bool {

		//re := regexp.MustCompile(`[0-9]{2}/[0-9]{2}/[0-9]{4} [0-9]{2}:[0-9]{2}`)
		//match := re.FindAllStringSubmatch(textToCheck, -1)
		//return len(match) > 0
		if (lastChar >= rune(47) && lastChar <= rune(58)) || lastChar == rune(32) {
			return true
		}
		return false

	}, func(date string) {
		card.DueDate = date
	})

	addForm.AddInputField("Order", "0", 5, func(textToCheck string, lastChar rune) bool {
		if lastChar < 48 || lastChar > 57 {
			return false
		}
		return true
	}, func(order string) {
		orderInt, _ := strconv.Atoi(order)
		card.Order = orderInt
	})

	return addForm, &card
}

func BuildDetailForm(card *deck_structs.Card) (*tview.Form, *deck_structs.Card) {
	addForm := tview.NewForm()
	addForm.SetTitle(" Edit Card Details ")
	addForm.SetBorder(true)
	addForm.SetBorderColor(utils.GetColor(configuration.Color))
	addForm.SetButtonBackgroundColor(utils.GetColor(configuration.Color))
	addForm.SetFieldBackgroundColor(tcell.ColorWhite)
	addForm.SetFieldTextColor(tcell.ColorBlack)
	addForm.SetLabelColor(utils.GetColor(configuration.Color))
	addForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			deck_ui.BuildFullFlex(deck_ui.MainFlex, nil)
			return nil
		}
		return event
	})
	addForm.AddInputField("Title", card.Title, 20, nil, func(title string) {
		card.Title = title
	})

	dueDate := ""
	if len(card.DueDate) > 0 {
		parse, _ := time.Parse("2006-01-02T15:04:05+00:00", card.DueDate)
		card.DueDate = parse.Format("02/01/2006 15:04")
		dueDate = fmt.Sprintf("%s", card.DueDate)
	}

	addForm.AddInputField("Due Date", dueDate, 18, func(textToCheck string, lastChar rune) bool {

		//re := regexp.MustCompile(`[0-9]{2}/[0-9]{2}/[0-9]{4} [0-9]{2}:[0-9]{2}`)
		//match := re.FindAllStringSubmatch(textToCheck, -1)
		//return len(match) > 0
		if (lastChar >= rune(47) && lastChar <= rune(58)) || lastChar == rune(32) {
			return true
		}
		return false

	}, func(date string) {
		card.DueDate = date
	})

	addForm.AddInputField("Order", strconv.Itoa(card.Order), 5, func(textToCheck string, lastChar rune) bool {
		if lastChar < 48 || lastChar > 57 {
			return false
		}
		return true
	}, func(order string) {
		orderInt, _ := strconv.Atoi(order)
		card.Order = orderInt
	})

	return addForm, card
}

func AddCard(actualList *tview.List, card deck_structs.Card) {
	var stackIndex, stack, _ = deck_stack.GetActualStack(actualList)

	jsonBody := fmt.Sprintf(`{"title":"%s", "description": "%s", "duedate": "%s", "type": "plain", "order": %d}`, utils.CleanText(card.Title), utils.CleanText(card.Description), card.DueDate, card.Order)
	var newCard deck_structs.Card
	var err error
	newCard, err = deck_http.AddCard(currentBoard.Id, stack.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error crating new card: %s", err.Error()))
		return
	}

	dueDate := ""
	if len(card.DueDate) > 0 {
		parse, _ := time.Parse("2006-01-02T15:04:05+00:00", newCard.DueDate)
		newCard.DueDate = parse.Format("02/01/2006 15:04")
		dueDate = fmt.Sprintf("(%s)", newCard.DueDate)
	}

	actualList.InsertItem(card.Order, fmt.Sprintf("[%s]#%d[white] - %s [red:-:-]%s[white]", configuration.Color, newCard.Id, newCard.Title, dueDate), "", rune(0), nil)
	CardsMap[newCard.Id] = newCard
	DetailText.Clear()
	EditableCard = newCard
	if deck_stack.Stacks[stackIndex].Cards == nil || len(deck_stack.Stacks[stackIndex].Cards) == 0 {
		deck_stack.Stacks[stackIndex].Cards = append(deck_stack.Stacks[stackIndex].Cards, newCard)
	} else {
		deck_stack.Stacks[stackIndex].Cards = append(deck_stack.Stacks[stackIndex].Cards[:1], deck_stack.Stacks[stackIndex].Cards[0:]...)
		deck_stack.Stacks[stackIndex].Cards[0] = newCard
	}
	DetailText.SetTitle(fmt.Sprintf(" %s ", newCard.Title))
	DetailText.SetText(utils.FormatDescription(newCard.Description))
	deck_ui.BuildFullFlex(DetailText, err)
}

func editCard() {
	description := utils.CleanText(EditableCard.Description)
	title := utils.CleanText(EditableCard.Title)
	dueDateFormat := ""
	if len(EditableCard.DueDate) > 0 {
		dueDateFormat = fmt.Sprintf(`,"duedate": "%s"`, EditableCard.DueDate)
	}
	jsonBody := fmt.Sprintf(`{"description": "%s", "title": "%s", "type": "plain", "owner":"%s"%s}`, utils.CleanText(description), utils.CleanText(title), configuration.User, dueDateFormat)
	var err error
	_, err = deck_http.UpdateCard(currentBoard.Id, EditableCard.StackId, EditableCard.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error updating card: %s", err.Error()))
	}
}

func updateCard(boardId, stackId int, cardId int, jsonBody string) {
	_, err := deck_http.UpdateCard(boardId, stackId, cardId, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error moving card: %s", err.Error()))
		return
	}
}

func DeleteCard(cardId int, stack deck_structs.Stack, actualList *tview.List, currentItemIndex int) {
	Modal.ClearButtons()
	Modal.SetText(fmt.Sprintf("Are you sure to delete card #%d?", cardId))
	Modal.SetBackgroundColor(utils.GetColor(configuration.Color))

	Modal.AddButtons([]string{"Yes", "No"})

	Modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			deck_ui.MainFlex.RemoveItem(Modal)
			app.SetFocus(actualList)
		}
		if event.Key() == tcell.KeyRight || event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyEnter {
			return event
		}
		return nil
	})

	Modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Yes" {
			go func() {
				_, err := deck_http.DeleteCard(currentBoard.Id, stack.Id, cardId, configuration)
				if err != nil {
					deck_ui.FooterBar.SetText(fmt.Sprintf("Error deleting card: %s", err.Error()))
				}
			}()
			actualList.RemoveItem(currentItemIndex)
			deck_ui.MainFlex.RemoveItem(Modal)
			app.SetFocus(actualList)
		} else if buttonLabel == "No" {
			deck_ui.MainFlex.RemoveItem(Modal)
			app.SetFocus(actualList)
		}
	})

	deck_ui.MainFlex.AddItem(Modal, 0, 0, false)
	app.SetFocus(Modal)
}

func AssignLabel(jsonBody string) {
	_, err := deck_http.AssignLabel(currentBoard.Id, EditableCard.StackId, EditableCard.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error assigning tag to card: %s", err.Error()))
	}
}

func DeleteLabel(jsonBody string) {
	_, err := deck_http.DeleteLabel(currentBoard.Id, EditableCard.StackId, EditableCard.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error deleting tag from card: %s", err.Error()))
	}
}
func AssignUser(jsonBody string) {
	_, err := deck_http.AssignUser(currentBoard.Id, EditableCard.StackId, EditableCard.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error assigning user to card: %s", err.Error()))
	}
}

func DeleteUser(jsonBody string) {
	_, err := deck_http.DeleteUser(currentBoard.Id, EditableCard.StackId, EditableCard.Id, jsonBody, configuration)
	if err != nil {
		deck_ui.FooterBar.SetText(fmt.Sprintf("Error deleting user from card: %s", err.Error()))
	}
}

func BuildStacks() {
	deck_ui.MainFlex.Clear()
	deck_ui.Primitives = make(map[tview.Primitive]int)
	deck_ui.PrimitivesIndexMap = make(map[int]tview.Primitive)

	sort.Slice(deck_stack.Stacks, func(i, j int) bool {
		return deck_stack.Stacks[i].Order < deck_stack.Stacks[j].Order
	})

	for index, s := range deck_stack.Stacks {
		todoList := tview.NewList()
		todoList.SetTitle(fmt.Sprintf(" %s ", s.Title))
		todoList.SetBorder(true)

		todoList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTAB {
				return nil
			}
			if event.Key() == tcell.KeyRight {
				if todoList.GetItemCount() == 0 {
					return nil
				}
				moveStackModal(todoList, tcell.KeyRight)
				return nil
			}
			if event.Key() == tcell.KeyLeft {
				if todoList.GetItemCount() == 0 {
					return nil
				}
				moveStackModal(todoList, tcell.KeyLeft)
				return nil
			}
			return event
		})

		sort.Slice(s.Cards, func(i, j int) bool {
			return s.Cards[i].Order < s.Cards[j].Order
		})

		for _, card := range s.Cards {
			var labels = utils.BuildLabels(card)
			CardsMap[card.Id] = card

			dueDate := ""
			if len(card.DueDate) > 0 {
				parse, _ := time.Parse("2006-01-02T15:04:05+00:00", card.DueDate)
				card.DueDate = parse.Format("02/01/2006 15:04")
				dueDate = fmt.Sprintf("- [red:-:-](%s)[white]", card.DueDate)
			}

			assigners := make([]string, 0)
			for _, o := range card.AssignedUsers {
				assigners = append(assigners, o.Participant.GetAbbrv())
			}

			assignersFormatter := ""
			if len(assigners) > 0 {
				assignersFormatter = fmt.Sprintf("- [red:gray:-]%s[-:-:-] ", utils.CommaString(assigners))
			}

			todoList.AddItem(fmt.Sprintf("[%s]#%d[white] %s- %s %s", configuration.Color, card.Id, assignersFormatter, card.Title, dueDate), labels, rune(0), nil)
		}

		todoList.SetSelectedFunc(func(index int, name string, secondName string, shortcut rune) {
			cardId := utils.GetId(name)

			DetailText.SetTitle(fmt.Sprintf(" #%d - %s ", CardsMap[cardId].Id, CardsMap[cardId].Title))
			DetailText.SetDynamicColors(true)

			description := utils.FormatDescription(CardsMap[cardId].Description)
			DetailText.SetText(deck_markdown.GetMarkDownDescription(description, configuration))
			EditableCard = CardsMap[cardId]
			deck_ui.BuildFullFlex(DetailText, nil)
		})

		todoList.SetFocusFunc(func() {
			todoList.SetTitleColor(utils.GetColor(configuration.Color))
		})

		deck_ui.Primitives[todoList] = index
		deck_ui.PrimitivesIndexMap[index] = todoList

		deck_ui.MainFlex.AddItem(todoList, 0, 1, true)
		primitive := deck_ui.MainFlex.GetItem(0)
		app.SetFocus(primitive)
	}
}

func moveStackModal(todoList *tview.List, key tcell.Key) {
	currentIndex := todoList.GetCurrentItem()
	currentText, _ := todoList.GetItemText(currentIndex)
	cardId := utils.GetId(currentText)

	primitive := app.GetFocus()

	Modal.ClearButtons()
	moveText := "next"
	if key == tcell.KeyLeft {
		moveText = "prev"
	}
	Modal.SetText(fmt.Sprintf("Are you sure to move card #%d to %s stack??", cardId, moveText))
	Modal.SetBackgroundColor(utils.GetColor(configuration.Color))

	Modal.AddButtons([]string{"Yes", "No"})

	Modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			deck_ui.MainFlex.RemoveItem(Modal)
			app.SetFocus(todoList)
		}
		if event.Key() == tcell.KeyRight || event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyEnter {
			return event
		}
		return nil
	})

	Modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Yes" {
			moveCardToStack(todoList, &primitive, key)
			deck_ui.MainFlex.RemoveItem(Modal)
		} else if buttonLabel == "No" {
			deck_ui.MainFlex.RemoveItem(Modal)
			app.SetFocus(todoList)
		}
	})
	deck_ui.MainFlex.AddItem(Modal, 0, 0, false)
	app.SetFocus(Modal)
}
