package catalog

import "testing"

func TestCleanTitleHint(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[PDF] A brief introduction to OpenCV | Semantic Scholar", "A brief introduction to OpenCV"},
		{"PDF Deep Reinforcement Learning with a Natural Language Action Space | Semantic Scholar", "Deep Reinforcement Learning with a Natural Language Action Space"},
		{"[HTML] Deep Learning for Computer Vision: A Brief Review", "Deep Learning for Computer Vision: A Brief Review"},
		{"2507.23357 Foundations and Models in Modern Computer Vision", "Foundations and Models in Modern Computer Vision"},
		{"[2212.09988] Multi-Reference Image Super-Resolution", "Multi-Reference Image Super-Resolution"},
		{"[2408.07712] Introduction to Reinforcement Learning - arXiv", "Introduction to Reinforcement Learning"},
		{"Multi-Reference Image Super-Resolution [2212.09988]", "Multi-Reference Image Super-Resolution"},
		{"Computer Vision - Algorithms and Applications | Semantic Scholar", "Computer Vision - Algorithms and Applications"},
		{"A review of deep-learning-based super-resolution - ScienceDirect.com", "A review of deep-learning-based super-resolution"},
		{"Machine Learning in Computer Vision - ScienceDirect", "Machine Learning in Computer Vision"},
		{"Computer Science - arXiv.org", "Computer Science"},
		{"Уроки компьютерного зрения. Оглавление / Хабр", "Уроки компьютерного зрения. Оглавление"},
		{"Осваиваем компьютерное зрение — 8 основных шагов - Habr", "Осваиваем компьютерное зрение — 8 основных шагов"},
		{"2201.09746 Конспект по обучению с подкреплением - ar5iv", "Конспект по обучению с подкреплением"},
		{"Deep Learning Empowered Super-Resolution: A Comprehensive ...", "Deep Learning Empowered Super-Resolution: A Comprehensive"},
		{"RL Token: Bootstrapping Online RL with Vision-Language …", "RL Token: Bootstrapping Online RL with Vision-Language"},
		{"  Attention   Is All\tYou Need  ", "Attention Is All You Need"},
		{"RL$^2$: Fast Reinforcement Learning via Slow ...", "RL$^2$: Fast Reinforcement Learning via Slow"},
		{"Editorial- Deep Learning for Computer Vision", "Editorial- Deep Learning for Computer Vision"},
		{"arXiv 2202.13124", ""},
		{"arXiv:2202.13124v2", ""},
		{"DOI 10.3390/rs12142207", ""},
		{"https://arxiv.org/abs/1511.08458", ""},
		{"www.semanticscholar.org/paper/abc", ""},
		{"arxiv.org/abs/1511.08458", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := cleanTitleHint(c.in); got != c.want {
			t.Errorf("cleanTitleHint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanTitleHintIsIdempotent(t *testing.T) {
	for _, in := range []string{"[PDF] A brief introduction to OpenCV | Semantic Scholar", "2507.23357 Foundations and Models in Modern Computer Vision"} {
		once := cleanTitleHint(in)
		if twice := cleanTitleHint(once); twice != once {
			t.Errorf("not idempotent: %q -> %q -> %q", in, once, twice)
		}
	}
}
