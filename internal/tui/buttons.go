package tui

// 各页面对应的底部按钮定义

func envButtons() []hotBtn {
	return []hotBtn{
		{"下一步", "enter"},
		{"返回", "esc"},
	}
}

func nameButtons() []hotBtn {
	return []hotBtn{
		{"下一步", "enter"},
		{"返回", "esc"},
	}
}

func sessionButtons() []hotBtn {
	return []hotBtn{
		{"下一步", "enter"},
		{"返回", "esc"},
	}
}

func confirmButtons() []hotBtn {
	return []hotBtn{
		{"确认创建", "enter"},
		{"返回", "esc"},
	}
}

func doneButtons() []hotBtn {
	return []hotBtn{
		{"退出", "enter"},
		{"回主页", "h"},
	}
}

func manageButtons() []hotBtn {
	return []hotBtn{
		{"进入", "enter"},
		{"删除", "d"},
		{"刷新", "r"},
		{"返回", "esc"},
	}
}

func envListButtons() []hotBtn {
	return []hotBtn{
		{"定位文件", "enter"},
		{"新建", "n"},
		{"返回", "esc"},
	}
}

func configButtons() []hotBtn {
	return []hotBtn{
		{"切换会话模式", "s"},
		{"切换自动删除", "a"},
		{"返回", "esc"},
	}
}
