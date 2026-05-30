package dailyquest

import (
	"time"
)

type Category string

const (
	CategoryMovementOutdoor   Category = "movement_outdoor"
	CategoryCreativeMake      Category = "creative_make"
	CategoryLearningCuriosity Category = "learning_curiosity"
	CategorySocialConnection  Category = "social_connection"
	CategoryReflectionReset   Category = "reflection_reset"
)

type Quest struct {
	ID            int
	Text          string
	FreeText      string
	Category      Category
	HighFrequency bool
}

type DailyQuest struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

var categoryOrder = []Category{
	CategoryMovementOutdoor,
	CategoryCreativeMake,
	CategoryLearningCuriosity,
	CategorySocialConnection,
	CategoryReflectionReset,
}

var categoryIDs = map[Category][]int{
	CategoryMovementOutdoor:   {1, 2, 6, 7, 9, 17, 35, 39, 49, 50},
	CategoryCreativeMake:      {5, 8, 10, 11, 12, 13, 14, 22, 23, 26, 28, 41, 42, 46},
	CategoryLearningCuriosity: {3, 16, 24, 27, 30, 34, 40, 43, 47},
	CategorySocialConnection:  {19, 21, 31, 44, 45},
	CategoryReflectionReset:   {4, 15, 25, 29, 32, 37, 38, 48},
}

var highFrequencyIDs = map[int]bool{
	3:  true,
	9:  true,
	26: true,
	29: true,
	31: true,
}

var questsByID = buildQuestMap()

func Select(now time.Time) []Quest {
	day := dateSeed(now)
	start := day % len(categoryOrder)
	selected := make([]Quest, 0, 3)
	usedCategories := make(map[Category]bool)
	usedIDs := make(map[int]bool)
	highFrequencyUsed := false

	for offset := 0; len(selected) < 3 && offset < len(categoryOrder)*2; offset++ {
		category := categoryOrder[(start+offset)%len(categoryOrder)]
		if usedCategories[category] {
			continue
		}
		quest, ok := selectFromCategory(category, day+offset*7, usedIDs, highFrequencyUsed)
		if !ok {
			continue
		}
		selected = append(selected, quest)
		usedCategories[category] = true
		usedIDs[quest.ID] = true
		if quest.HighFrequency {
			highFrequencyUsed = true
		}
	}
	return selected
}

func QuestByID(id int) (Quest, bool) {
	quest, ok := questsByID[id]
	return quest, ok
}

func SafeText(quest Quest) string {
	if quest.FreeText != "" {
		return quest.FreeText
	}
	return quest.Text
}

func Fallback(quests []Quest) []DailyQuest {
	result := make([]DailyQuest, 0, len(quests))
	for _, quest := range quests {
		if quest.ID == 0 || SafeText(quest) == "" {
			continue
		}
		result = append(result, DailyQuest{ID: quest.ID, Text: SafeText(quest)})
	}
	return result
}

func IsValidGenerated(quests []Quest, generated []DailyQuest) bool {
	if len(quests) != len(generated) || len(generated) == 0 {
		return false
	}
	expected := make(map[int]bool, len(quests))
	for _, quest := range quests {
		expected[quest.ID] = true
	}
	seen := make(map[int]bool, len(generated))
	for _, quest := range generated {
		if !expected[quest.ID] || seen[quest.ID] || quest.Text == "" {
			return false
		}
		seen[quest.ID] = true
	}
	return true
}

func selectFromCategory(category Category, seed int, usedIDs map[int]bool, highFrequencyUsed bool) (Quest, bool) {
	ids := categoryIDs[category]
	if len(ids) == 0 {
		return Quest{}, false
	}
	start := seed % len(ids)
	for step := 0; step < len(ids); step++ {
		id := ids[(start+step)%len(ids)]
		quest, ok := QuestByID(id)
		if !ok || usedIDs[id] {
			continue
		}
		if (id == 1 && usedIDs[50]) || (id == 50 && usedIDs[1]) {
			continue
		}
		if highFrequencyUsed && quest.HighFrequency {
			continue
		}
		return quest, true
	}
	return Quest{}, false
}

func dateSeed(now time.Time) int {
	location, err := time.LoadLocation("Europe/Minsk")
	if err == nil {
		now = now.In(location)
	}
	year, day := now.ISOWeek()
	return year*371 + day*7 + now.YearDay()
}

func buildQuestMap() map[int]Quest {
	result := map[int]Quest{}
	for _, quest := range canonicalQuests() {
		quest.HighFrequency = highFrequencyIDs[quest.ID]
		result[quest.ID] = quest
	}
	for category, ids := range categoryIDs {
		for _, id := range ids {
			quest := result[id]
			quest.Category = category
			quest.HighFrequency = highFrequencyIDs[id]
			result[id] = quest
		}
	}
	return result
}

func canonicalQuests() []Quest {
	return []Quest{
		{ID: 1, Text: "Проснуться до рассвета и прогуляться без наушников."},
		{ID: 2, Text: "Сходить одному на кофе без телефона.", FreeText: "Зайди в лобби, библиотеку или другое бесплатное публичное пространство и посиди 10 минут без телефона."},
		{ID: 3, Text: "Выучить 10 созвездий и найти их на небе."},
		{ID: 4, Text: "Написать письмо, которое никогда не отправишь."},
		{ID: 5, Text: "Приготовить блюдо из страны, где не был."},
		{ID: 6, Text: "Почитать бумажную книгу на улице."},
		{ID: 7, Text: "Составить карту любимых мест в своём районе."},
		{ID: 8, Text: "Сделать хлеб или пасту вручную."},
		{ID: 9, Text: "Пройти привычный маршрут в другое время."},
		{ID: 10, Text: "Завести небольшой огород с травами."},
		{ID: 11, Text: "Рисовать обычные предметы по 20 минут в день."},
		{ID: 12, Text: "Засушить листья или цветы."},
		{ID: 13, Text: "Неделю заниматься каллиграфией."},
		{ID: 14, Text: "Собрать плейлист для этого сезона своей жизни."},
		{ID: 15, Text: "Написать дневник в кафе и наблюдать за людьми.", FreeText: "Напиши дневник на лавочке, в библиотеке или в лобби и 10 минут спокойно понаблюдай за людьми."},
		{ID: 16, Text: "Выучить один приём самообороны."},
		{ID: 17, Text: "Потянуть тело по-новому."},
		{ID: 18, Text: "Сходить одному в музей.", FreeText: "Посети бесплатную экспозицию, галерею, публичное пространство или виртуальную коллекцию."},
		{ID: 19, Text: "Сделать искренний комплимент незнакомцу."},
		{ID: 20, Text: "Сходить на открытую лекцию.", FreeText: "Посмотри бесплатную лекцию онлайн или найди бесплатное публичное мероприятие."},
		{ID: 21, Text: "Стать волонтёром один раз и не выкладывать это в соцсети."},
		{ID: 22, Text: "Сделать что-нибудь своими руками."},
		{ID: 23, Text: "Починить одну вещь у себя дома."},
		{ID: 24, Text: "Посмотреть классический фильм от начала до конца."},
		{ID: 25, Text: "Устроить 24 часа без соцсетей."},
		{ID: 26, Text: "Сфотографировать интересные текстуры вокруг себя."},
		{ID: 27, Text: "Выучить наизусть короткое стихотворение."},
		{ID: 28, Text: "Научиться готовить один идеальный соус."},
		{ID: 29, Text: "Посидеть в тишине 10 минут."},
		{ID: 30, Text: "Освоить базу первой помощи."},
		{ID: 31, Text: "Вместо болтовни задать один настоящий вопрос."},
		{ID: 32, Text: "Провести целый день и не тратить деньги."},
		{ID: 33, Text: "Начать делать компост."},
		{ID: 34, Text: "Записывать фразы, которые случайно слышишь в общественных местах."},
		{ID: 35, Text: "Исследовать новый район города."},
		{ID: 36, Text: "Попробовать новое занятие (бокс, гончарное дело, танцы).", FreeText: "Сделай бесплатный вводный урок дома по видео на 20 минут: танцы, бокс-движения или лепка из подручного."},
		{ID: 37, Text: "Перечитать старые записи в дневнике."},
		{ID: 38, Text: "Придумать альтернативный план жизни на 5 лет."},
		{ID: 39, Text: "Сначала подвигать, а потом брать телефон."},
		{ID: 40, Text: "Выучить пять полезных навыков."},
		{ID: 41, Text: "Начать небольшой творческий проект."},
		{ID: 42, Text: "Поужинать при свечах."},
		{ID: 43, Text: "Изучить базовый язык тела."},
		{ID: 44, Text: "Спросить родителей про их молодые годы."},
		{ID: 45, Text: "Сделать что-то социально неловкое."},
		{ID: 46, Text: "Разобрать и выбросить 50 лишних вещей."},
		{ID: 47, Text: "Выучить наизусть любимую речь."},
		{ID: 48, Text: "Прожить 24 часа без жалоб."},
		{ID: 49, Text: "Запланировать маленькое приключение на выходные."},
		{ID: 50, Text: "Проснуться до рассвета и прогуляться без телефона."},
	}
}
