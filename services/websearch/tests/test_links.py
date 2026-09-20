from datetime import date

import pytest
from fastapi.testclient import TestClient

import app.main as main
from app.main import (
    Source,
    answer_cites_sources,
    has_linked_sources_section,
    is_non_paper_url,
    markdown_sources,
    merge_sources,
    system_prompt,
)

# Links users actually got in production answers, plus the index-page classes
# the model was seen inventing in search answers.
INDEX_PAGES = [
    "https://arxiv.org/",
    "https://arxiv.org/login",
    "https://arxiv.org/archive/cs.CV",
    "https://arxiv.org/list/cs.CV/recent",
    "https://arxiv.org/list/cs.CV/current",
    "https://arxiv.org/list/cs.LG/new",
    "https://www.arxiv.org/list/cs.CV/recent?skip=111&show=100",
    "https://export.arxiv.org/list/cs.CV/new",
    "https://arxiv.org/catchup/cs.CV/2026-09-01",
    "https://arxiv.org/year/cs/2024",
    "https://arxiv.org/search/?query=super+resolution&searchtype=all",
    "https://arxiv.org/a/szeliski_r_1",
    "https://arxiv.org/help/api",
    "https://info.arxiv.org/help/find/index.html",
    "https://blog.arxiv.org/",
    "https://blog.arxiv.org/2026/",
    "https://status.arxiv.org/",
    "https://search.arxiv.org/",
    "https://openreview.net/",
    "https://openreview.net/?id=S1ecm2C9K",
    "https://openreview.net/venues",
    "https://openreview.net/venue?id=ACM.org",
    "https://openreview.net/group?id=ICLR.cc/2024/Conference",
    "https://www.semanticscholar.org/",
    "https://www.semanticscholar.org/topic/Computer-vision/5332",
    "https://www.semanticscholar.org/product",
    "https://www.semanticscholar.org/search?q=opencv",
    "https://www.semanticscholar.org/author/Richard-Szeliski/1717841",
    "https://www.semanticscholar.org/venue?name=CVPR",
    "https://www.sciencedirect.com/",
    "https://www.sciencedirect.com/journal/computer-vision-and-image-understanding/vol/256/suppl/C",
    "https://www.sciencedirect.com/browse/journals-and-books",
    "https://proceedings.neurips.cc/paper_files/paper/2024",
    "https://proceedings.neurips.cc/paper_files/paper/2023/",
    "https://proceedings.neurips.cc/paper/2022",
    "https://papers.nips.cc/paper/2019",
    "https://papers.nips.cc/",
    "https://openaccess.thecvf.com/menu",
    "https://openaccess.thecvf.com/menu_other.html",
    "https://openaccess.thecvf.com/ICCV2021_workshops/menu",
    "https://openaccess.thecvf.com/ICCV2025?day=all",
    "https://aclanthology.org/",
    "https://aclanthology.org/events/acl-2024/",
    "https://aclanthology.org/venues/acl/",
    "https://aclanthology.org/volumes/2020.acl-main/",
    "https://proceedings.mlr.press/v139/",
    "https://doi.org/",
    "https://example.com/index.html",
    # Classes shared with backend catalog/indexpages.go.
    "https://labs.arxiv.org/",
    "https://confluence.arxiv.org/display/DOC",
    "https://scholar.google.com/scholar?q=opencv",
    "https://scholar.google.ru/citations?user=abc",
    "https://arxiv.org/submit",
    "https://openreview.net/tasks",
    "https://www.semanticscholar.org/alerts",
    "https://www.sciencedirect.com/user/login",
    "https://aclanthology.org/posts/2024-01-01-news/",
    "https://ar5iv.labs.arxiv.org/log/2024",
    "https://proceedings.neurips.cc/paper_files/paper",
    "https://proceedings.neurips.cc/admin/login",
    "https://jmlr.org/papers/",
    "https://jmlr.org/papers/v21/",
    "https://ieeexplore.ieee.org/search/searchresult.jsp?queryText=vision",
    "https://www.nature.com/search?q=vision",
    "https://ijcai.org/index.php",
]

PAPER_PAGES = [
    "https://arxiv.org/abs/1511.08458",
    "http://arxiv.org/abs/2007.03107",
    "https://www.arxiv.org/abs/2412.09846",
    "https://arxiv.org/pdf/2202.13124",
    "https://arxiv.org/html/2509.22692v1",
    "https://ar5iv.labs.arxiv.org/html/2201.09746",
    "https://doi.org/10.1109/LGRS.2019.2940483",
    "https://doi.org/10.1007/978-1-84882-935-0",
    "https://www.semanticscholar.org/paper/A-brief-introduction-to-OpenCV-%C4%8Culjak-Abram/3356363c5857414591b0bf9c20544cc7e88d2cfb",
    "https://openreview.net/forum?id=kAHhFAoZtk",
    "https://openreview.net/pdf?id=kAHhFAoZtk",
    "https://www.sciencedirect.com/science/article/abs/pii/S0031320324006861",
    "https://www.sciencedirect.com/org/science/article/pii/S1526149223001182",
    "https://www.sciencedirect.com/science/article/pii/S1877050920308218/pdf?md5=9ced58aea1b17ed22bcb823e29640ebb&pid=1-s2.0-S1877050920308218-main.pdf",
    "https://openaccess.thecvf.com/content/CVPR2022W/PBVS/papers/Ibrahim_3DRRDB_Super_Resolution_of_Multiple_Remote_Sensing_Images_Using_3D_CVPRW_2022_paper.pdf",
    "https://openaccess.thecvf.com/content_cvpr_2017/html/Huang_Densely_Connected_Convolutional_CVPR_2017_paper.html",
    "https://proceedings.neurips.cc/paper_files/paper/2024/hash/0123456789abcdef-Abstract-Conference.html",
    "https://papers.nips.cc/paper/2017/hash/3f5ee243547dee91fbd053c1c4a845aa-Abstract.html",
    "https://papers.nips.cc/paper/7181-attention-is-all-you-need",
    "https://aclanthology.org/2020.acl-main.1/",
    "https://aclanthology.org/2020.acl-main.1.pdf",
    "https://proceedings.mlr.press/v139/radford21a.html",
    "https://ojs.aaai.org/index.php/AAAI/article/view/16826",
    "https://ieeexplore.ieee.org/document/10909490/",
    "https://www.nature.com/articles/s41598-025-93049-7",
    "https://jmlr.org/papers/v21/20-074.html",
    "https://ar5iv.org/abs/1706.03762",
    "https://proceedings.neurips.cc/paper/2017/file/3f5ee243547dee91fbd053c1c4a845aa-Paper.pdf",
    "https://pubmed.ncbi.nlm.nih.gov/29487619/",
]


@pytest.mark.parametrize("url", INDEX_PAGES)
def test_index_pages_are_not_papers(url: str) -> None:
    assert is_non_paper_url(url)


@pytest.mark.parametrize("url", PAPER_PAGES)
def test_paper_pages_are_kept(url: str) -> None:
    assert not is_non_paper_url(url)


def test_non_urls_are_left_to_url_validation() -> None:
    assert not is_non_paper_url("not-a-url")
    assert not is_non_paper_url("")


def test_merge_sources_drops_every_index_page() -> None:
    sources: list[Source] = []
    merge_sources(sources, [*INDEX_PAGES, *PAPER_PAGES])
    assert [source.url for source in sources] == PAPER_PAGES


def test_markdown_sources_skips_index_pages() -> None:
    rendered = markdown_sources(
        [
            Source(title="arXiv cs.CV", url="https://arxiv.org/list/cs.CV/recent", domain="arxiv.org"),
            Source(
                title="An Introduction to Convolutional Neural Networks",
                url="https://arxiv.org/abs/1511.08458",
                domain="arxiv.org",
            ),
        ]
    )
    assert rendered == (
        "\n\n## Sources\n\n"
        "1. [An Introduction to Convolutional Neural Networks](https://arxiv.org/abs/1511.08458)"
    )
    assert markdown_sources(
        [Source(title="OpenReview", url="https://openreview.net/", domain="openreview.net")]
    ) == ""


def test_sources_section_of_index_pages_only_is_not_enough() -> None:
    only_index = (
        "Answer\n\n## Источники\n\n"
        "[Advances in Neural Information Processing Systems 37]"
        "(https://proceedings.neurips.cc/paper_files/paper/2024)"
    )
    assert not has_linked_sources_section(only_index)
    with_paper = only_index + "\n[[1706.03762] Attention Is All You Need](https://arxiv.org/abs/1706.03762)"
    assert has_linked_sources_section(with_paper)


def test_small_talk_does_not_cite_sources() -> None:
    assert not answer_cites_sources("Привет! Какой исследовательский вопрос разберём?")
    assert answer_cites_sources("Метод описан в обзоре [1].")
    assert answer_cites_sources("[An Introduction to CNNs](https://arxiv.org/abs/1511.08458)")


def stream_text(monkeypatch, events: list[tuple[str, object]], mode: str = "web") -> str:
    async def fake_provider_stream(_payload):
        for event in events:
            yield event

    monkeypatch.setattr(main, "provider_stream", fake_provider_stream)
    response = TestClient(main.app).post(
        "/v1/search/stream",
        json={"messages": [{"role": "user", "content": "question"}], "mode": mode},
    )
    assert response.status_code == 200
    return response.text


def test_stream_payloads_never_carry_index_pages(monkeypatch) -> None:
    text = stream_text(
        monkeypatch,
        [
            (
                "sources",
                [
                    "https://arxiv.org/list/cs.CV/recent",
                    {"title": "OpenReview venues", "url": "https://openreview.net/venues"},
                    {"title": "An Introduction to CNNs", "url": "https://arxiv.org/abs/1511.08458"},
                ],
            ),
            ("delta", "Обзор свёрточных сетей [1]."),
        ],
        mode="deep",
    )
    assert "arxiv.org/list" not in text
    assert "openreview.net/venues" not in text
    assert 'event: source_progress\ndata: {"count": 1,' in text
    assert 'event: sources\ndata: {"sources": [{"title": "An Introduction to CNNs"' in text
    assert "1. [An Introduction to CNNs](https://arxiv.org/abs/1511.08458)" in text
    assert 'event: done\ndata: {"status": "ok"}' in text


def test_small_talk_answer_gets_no_sources_section(monkeypatch) -> None:
    text = stream_text(
        monkeypatch,
        [
            ("sources", [{"title": "привет", "url": "https://ru.wiktionary.org/wiki/привет"}]),
            ("delta", "Привет! Какой исследовательский вопрос разберём?"),
        ],
    )
    assert "## Sources" not in text
    assert 'event: done\ndata: {"status": "ok"}' in text


def test_prompt_restricts_links_to_specific_papers() -> None:
    prompt = system_prompt("web", current_date=date(2026, 9, 15))
    assert "arxiv.org/list/" in prompt
    assert "proceedings.neurips.cc/paper_files/paper/2024" in prompt
    assert "Текст ссылки — точное оригинальное название статьи" in prompt
    assert "Указывай arXiv ID только если он взят из результата поиска именно для этой статьи" in prompt
    assert "приветствие" in prompt


def test_indexed_citations_keep_provider_positions() -> None:
    values = main.indexed_citations(
        ["https://arxiv.org/list/cs.CV/recent", {"title": "CNNs", "url": "https://arxiv.org/abs/1511.08458"}]
    )
    assert values == [
        {"url": "https://arxiv.org/list/cs.CV/recent", "citation_index": 1},
        {"title": "CNNs", "url": "https://arxiv.org/abs/1511.08458", "citation_index": 2},
    ]
    assert main.indexed_citations(None) == []


def test_sources_keep_citation_numbers_after_dropping_index_pages() -> None:
    sources: list[Source] = []
    merge_sources(
        sources,
        main.indexed_citations(
            [
                "https://arxiv.org/abs/1706.03762",
                "https://arxiv.org/list/cs.CV/recent",
                {"title": "An Introduction to CNNs", "url": "https://arxiv.org/abs/1511.08458"},
            ]
        ),
    )
    assert [source.citation for source in sources] == [1, 3]
    # The number is internal: the sources payload is unchanged.
    assert "citation" not in sources[0].model_dump()
    rendered = markdown_sources(sources)
    # [3] in the answer text must still name the third provider citation.
    assert rendered == (
        "\n\n## Sources\n\n"
        "1\\. [arxiv.org](https://arxiv.org/abs/1706.03762)\n\n"
        "3\\. [An Introduction to CNNs](https://arxiv.org/abs/1511.08458)"
    )


def test_repeated_citation_lists_do_not_duplicate_sources() -> None:
    sources: list[Source] = []
    chunk = ["https://arxiv.org/abs/1706.03762", "https://arxiv.org/abs/1511.08458"]
    merge_sources(sources, main.indexed_citations(chunk))
    merge_sources(sources, main.indexed_citations(chunk))
    assert [(source.url, source.citation) for source in sources] == [
        ("https://arxiv.org/abs/1706.03762", 1),
        ("https://arxiv.org/abs/1511.08458", 2),
    ]
    assert markdown_sources(sources).endswith("2. [arxiv.org](https://arxiv.org/abs/1511.08458)")
