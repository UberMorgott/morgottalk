// Package i18n provides backend-side translations for system tray,
// native dialogs, and other Go-visible user-facing strings.
// Frontend translations live in frontend/src/lib/i18n.ts.
package i18n

// T returns the localized string for the given language and key.
// Falls back to English if the language or key is not found.
func T(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := translations["en"][key]; ok {
		return v
	}
	return key
}

var translations = map[string]map[string]string{
	"en": {
		"tray_show":            "Show",
		"tray_quit":            "Quit",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "What would you like to do when closing the window?",
		"close_minimize":       "Minimize to tray",
		"close_quit":           "Quit",
	},
	"ru": {
		"tray_show":            "Показать",
		"tray_quit":            "Выход",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "Что сделать при закрытии окна?",
		"close_minimize":       "Свернуть в трей",
		"close_quit":           "Выход",
	},
	"de": {
		"tray_show":            "Anzeigen",
		"tray_quit":            "Beenden",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "Was möchten Sie beim Schließen des Fensters tun?",
		"close_minimize":       "In den Tray minimieren",
		"close_quit":           "Beenden",
	},
	"es": {
		"tray_show":            "Mostrar",
		"tray_quit":            "Salir",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "¿Qué desea hacer al cerrar la ventana?",
		"close_minimize":       "Minimizar a la bandeja",
		"close_quit":           "Salir",
	},
	"fr": {
		"tray_show":            "Afficher",
		"tray_quit":            "Quitter",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "Que souhaitez-vous faire en fermant la fenêtre ?",
		"close_minimize":       "Réduire dans la barre",
		"close_quit":           "Quitter",
	},
	"zh": {
		"tray_show":            "显示",
		"tray_quit":            "退出",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "关闭窗口时您想做什么？",
		"close_minimize":       "最小化到托盘",
		"close_quit":           "退出",
	},
	"ja": {
		"tray_show":            "表示",
		"tray_quit":            "終了",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "ウィンドウを閉じるときの動作を選択してください",
		"close_minimize":       "トレイに最小化",
		"close_quit":           "終了",
	},
	"pt": {
		"tray_show":            "Mostrar",
		"tray_quit":            "Sair",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "O que deseja fazer ao fechar a janela?",
		"close_minimize":       "Minimizar para a bandeja",
		"close_quit":           "Sair",
	},
	"ko": {
		"tray_show":            "표시",
		"tray_quit":            "종료",
		"close_dialog_title":   "MorgoTTalk",
		"close_dialog_message": "창을 닫을 때 어떻게 하시겠습니까?",
		"close_minimize":       "트레이로 최소화",
		"close_quit":           "종료",
	},
}
