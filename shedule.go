package gomaishedule

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	sheduleURL = "https://mai.ru/education/schedule/"
	detailURL  = sheduleURL + "detail.php?group="
	sessionURL = sheduleURL + "session.php?group="
)

//Shedule is representation of group and its schedule for certain period
type Shedule struct {
	Group       string
	ThisWeekNum int
	Shedule     []*StudyDay
	Weeks       []*StudyWeek
}

//StudyWeek is representation of number of the week and its boundary
type StudyWeek struct {
	Num     int
	Borders string
}

//StudyDay is representation of day as set of pairs and description of day
type StudyDay struct {
	Date      string
	WeekDay   string
	PairCount int
	Pairs     []*Pair
}

//Pair is representation of pair and its parameters
type Pair struct {
	Time     string
	Type     string
	Title    string
	Lecturer string
	Location string
}

//ValidateGroup checks compliance of group with pattern and existence in list of schedule groups.
//Returns errCode '1' if the pattern does not match and '2' if the group does not exist and '0' if succesfull.
func ValidateGroup(group string) (int, error) {
	if match, _ := regexp.MatchString("-\\d{1,3}\\s{0,1}\\p{L}{1,3}-\\d{1,}", group); !match {
		return 1, errors.New("Group '" + group + "' doesn't match the pattern")
	}

	page, err := getPage(sheduleURL)
	if err != nil {
		return 10, err
	}
	isErr := true
	page.Find("div#schedule-content").Each(func(index int, sheduleContent *goquery.Selection) {
		if strings.Contains(sheduleContent.Find("a.sc-group-item").Text(), group) {
			isErr = false
			return
		}
	})
	if isErr {
		return 2, errors.New("Group '" + group + "' not found in MAI shedule")
	}
	return 0, nil
}

//GetDayShedule returns schedule pairs for day specified in parseType for specified group
func GetDayShedule(parseType, group string) (*Shedule, error) {
	errCode, err := ValidateGroup(group)
	if err != nil {
		return &Shedule{}, errors.New("Error code: " + strconv.Itoa(errCode) + ", " + err.Error())
	}

	var daysCount int
	var date string
	switch parseType {
	case "today":
		daysCount = 1
		date = time.Now().Format("02.01")
	case "tomorrow":
		daysCount = 2
		date = time.Now().Add(24 * time.Hour).Format("02.01")
	default:
		return &Shedule{}, errors.New("Wrong parseType '" + parseType + "'")
	}

	url := detailURL + group
	page, err := getPage(url)
	if err != nil {
		return &Shedule{}, err
	}

	studyDays := parseStudyDays(page, daysCount)
	shedule := &Shedule{}
	shedule.Shedule = make([]*StudyDay, 1)
	for _, studyDay := range studyDays {
		if studyDay.Date == date {
			
			shedule.Shedule[0] = studyDay
		}
	}
	fmt.Println(shedule.Shedule[0])
	return shedule, nil
}

//GetWeekShedule returns schedule pairs for week specified in parseType for specified group
func GetWeekShedule(parseType, group string) (*Shedule, error) {
	errCode, err := ValidateGroup(group)
	if err != nil {
		return &Shedule{}, errors.New("Error code: " + strconv.Itoa(errCode) + ", " + err.Error())
	}

	url := detailURL + group
	page, err := getPage(url)
	if err != nil {
		return &Shedule{}, err
	}

	shedule := &Shedule{}
	shedule.Weeks, shedule.ThisWeekNum = parseStudyWeeks(page)

	if shedule.ThisWeekNum == 0 {
		return shedule, errors.New("Current week doesn't correspond to any shedule week")
	}

	switch parseType {
	case "thisweeknum":
		return shedule, nil
	case "thisweek":
		url += "&week=" + strconv.Itoa(shedule.ThisWeekNum)
	case "nextweek":
		weekNum := shedule.ThisWeekNum + 1
		url += "&week=" + strconv.Itoa(weekNum)
	default:
		exist, _ := regexp.MatchString("\\d{1,2}week", parseType)
		switch exist {
		case true:
			url += "&week=" + strings.Trim(parseType, "week")
		case false:
			return &Shedule{}, errors.New("Wrong ParseType '" + parseType + "'")
		}
	}

	page, err = getPage(url)
	if err != nil {
		return &Shedule{}, err
	}

	shedule.Shedule = parseStudyDays(page, 0)
	return shedule, nil
}

//GetSessionShedule returns session schedule for specified group
func GetSessionShedule(group string) (*Shedule, error) {
	errCode, err := ValidateGroup(group)
	if err != nil {
		return &Shedule{}, errors.New("Error code: " + strconv.Itoa(errCode) + ", " + err.Error())
	}

	url := sessionURL + group
	page, err := getPage(url)
	if err != nil {
		return &Shedule{}, err
	}

	studyDays := parseStudyDays(page, 0)
	shedule := &Shedule{}
	shedule.Shedule = studyDays
	return shedule, nil
}

func parseStudyDays(page *goquery.Document, daysCount int) []*StudyDay {
	re := regexp.MustCompile(`.{1,5}`)
	daysContainer := page.Find("div#schedule-content").Find("div.sc-container")
	if daysCount == 0 { //for weeks and session parsing
		daysCount = daysContainer.Length()
	}
	shedule := make([]*StudyDay, daysCount)
	daysContainer.EachWithBreak(func(dayIndex int, dayContainer *goquery.Selection) bool {
		if dayIndex < daysCount {
			dayHeader := dayContainer.Find("div.sc-table-col.sc-day-header").Text()
			date := re.FindAllString(dayHeader, -1)
			shedule[dayIndex] = &StudyDay{}
			shedule[dayIndex].Date = date[0]
			shedule[dayIndex].WeekDay = date[1]
			pairString := dayContainer.Find("div.sc-table.sc-table-detail").Find("div.sc-table-row")
			shedule[dayIndex].PairCount = pairString.Length()
			shedule[dayIndex].Pairs = make([]*Pair, shedule[dayIndex].PairCount)
			pairString.Each(func(pairIndex int, pair *goquery.Selection) {
				shedule[dayIndex].Pairs[pairIndex] = &Pair{}
				shedule[dayIndex].Pairs[pairIndex].Time = pair.Find("div.sc-table-col.sc-item-time").Text()
				shedule[dayIndex].Pairs[pairIndex].Type = pair.Find("div.sc-table-col.sc-item-type").Text()
				shedule[dayIndex].Pairs[pairIndex].Title = pair.Find("span.sc-title").Text()
				shedule[dayIndex].Pairs[pairIndex].Lecturer = pair.Find("span.sc-lecturer").Text()
				shedule[dayIndex].Pairs[pairIndex].Location = pair.Find("div.sc-table-col.sc-item-location").Text()
			})
			return true
		} else {
			return false
		}
	})
	return shedule
}

func parseStudyWeeks(page *goquery.Document) ([]*StudyWeek, int) {
	weeksStrings := page.Find("div#schedule-content").Find("table.table").Find("tr").Find("td")
	weeks := make([]*StudyWeek, weeksStrings.Length()/2)

	year := time.Now().Format("2006")
	yearMin := time.Now().Format("06")
	i, j := 0, 0

	weeksStrings.Each(func(weekIndex int, week *goquery.Selection) {
		switch weekIndex % 2 {
		case 0:
			n, _ := strconv.Atoi(week.Text())
			weeks[weekIndex-i] = &StudyWeek{}
			weeks[weekIndex-i].Num = n
			i++
		case 1:
			j++
			borders := strings.ReplaceAll(week.Text(), " - ", "-")
			borders = strings.ReplaceAll(borders, year, yearMin)
			weeks[weekIndex-j].Borders = borders
		}
	})

	var thisWeekNum int
	for _, week := range weeks {
		days := strings.Split(week.Borders, "-")
		firstDate, _ := time.Parse("02.01.06", days[0])
		secondDate, _ := time.Parse("02.01.06", days[1])
		if time.Now().After(firstDate) && time.Now().Before(secondDate) {
			thisWeekNum = week.Num
			break
		}
	}
	return weeks, thisWeekNum
}

func getPage(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return &goquery.Document{}, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return &goquery.Document{}, err
	}
	return doc, nil
}
