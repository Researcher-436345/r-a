--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: alembic_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);


--
-- Name: annotations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.annotations (
    id uuid NOT NULL,
    paper_id uuid NOT NULL,
    user_id uuid NOT NULL,
    page integer NOT NULL,
    rect jsonb,
    selected_text text NOT NULL,
    note text NOT NULL,
    color character varying(16) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: authors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.authors (
    id uuid NOT NULL,
    name character varying(500) NOT NULL,
    normalized_name character varying(500) NOT NULL,
    orcid character varying(32)
);


--
-- Name: paper_authors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.paper_authors (
    id uuid NOT NULL,
    paper_id uuid NOT NULL,
    author_id uuid NOT NULL,
    "position" integer NOT NULL
);


--
-- Name: paper_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.paper_versions (
    id uuid NOT NULL,
    paper_id uuid NOT NULL,
    version_number integer NOT NULL,
    source character varying(64) NOT NULL,
    source_url character varying(1000),
    pdf_key character varying(500),
    sha256 character varying(64),
    size_bytes bigint,
    status character varying(32) NOT NULL,
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: papers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.papers (
    id uuid NOT NULL,
    title character varying(1000) NOT NULL,
    abstract text,
    year integer,
    venue character varying(500),
    doi character varying(255),
    arxiv_id character varying(64),
    language character varying(16),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_library_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_library_items (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    paper_id uuid NOT NULL,
    status character varying(32) NOT NULL,
    favorite boolean NOT NULL,
    added_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid NOT NULL,
    email character varying(320) NOT NULL,
    password_hash character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: alembic_version alembic_version_pkc; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);


--
-- Name: annotations annotations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.annotations
    ADD CONSTRAINT annotations_pkey PRIMARY KEY (id);


--
-- Name: authors authors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.authors
    ADD CONSTRAINT authors_pkey PRIMARY KEY (id);


--
-- Name: paper_authors paper_authors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_authors
    ADD CONSTRAINT paper_authors_pkey PRIMARY KEY (id);


--
-- Name: paper_versions paper_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_versions
    ADD CONSTRAINT paper_versions_pkey PRIMARY KEY (id);


--
-- Name: papers papers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.papers
    ADD CONSTRAINT papers_pkey PRIMARY KEY (id);


--
-- Name: paper_authors uq_paper_author; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_authors
    ADD CONSTRAINT uq_paper_author UNIQUE (paper_id, author_id);


--
-- Name: user_library_items uq_user_paper; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_library_items
    ADD CONSTRAINT uq_user_paper UNIQUE (user_id, paper_id);


--
-- Name: user_library_items user_library_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_library_items
    ADD CONSTRAINT user_library_items_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: ix_annotations_paper_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_annotations_paper_id ON public.annotations USING btree (paper_id);


--
-- Name: ix_annotations_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_annotations_user_id ON public.annotations USING btree (user_id);


--
-- Name: ix_authors_normalized_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_authors_normalized_name ON public.authors USING btree (normalized_name);


--
-- Name: ix_paper_authors_author_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_paper_authors_author_id ON public.paper_authors USING btree (author_id);


--
-- Name: ix_paper_authors_paper_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_paper_authors_paper_id ON public.paper_authors USING btree (paper_id);


--
-- Name: ix_paper_versions_paper_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_paper_versions_paper_id ON public.paper_versions USING btree (paper_id);


--
-- Name: ix_paper_versions_sha256; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_paper_versions_sha256 ON public.paper_versions USING btree (sha256);


--
-- Name: ix_papers_arxiv_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_papers_arxiv_id ON public.papers USING btree (arxiv_id);


--
-- Name: ix_papers_doi; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_papers_doi ON public.papers USING btree (doi);


--
-- Name: ix_user_library_items_paper_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_library_items_paper_id ON public.user_library_items USING btree (paper_id);


--
-- Name: ix_user_library_items_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_library_items_user_id ON public.user_library_items USING btree (user_id);


--
-- Name: ix_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX ix_users_email ON public.users USING btree (email);


--
-- Name: annotations annotations_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.annotations
    ADD CONSTRAINT annotations_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES public.papers(id) ON DELETE CASCADE;


--
-- Name: annotations annotations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.annotations
    ADD CONSTRAINT annotations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: paper_authors paper_authors_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_authors
    ADD CONSTRAINT paper_authors_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.authors(id) ON DELETE CASCADE;


--
-- Name: paper_authors paper_authors_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_authors
    ADD CONSTRAINT paper_authors_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES public.papers(id) ON DELETE CASCADE;


--
-- Name: paper_versions paper_versions_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.paper_versions
    ADD CONSTRAINT paper_versions_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES public.papers(id) ON DELETE CASCADE;


--
-- Name: user_library_items user_library_items_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_library_items
    ADD CONSTRAINT user_library_items_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES public.papers(id) ON DELETE CASCADE;


--
-- Name: user_library_items user_library_items_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_library_items
    ADD CONSTRAINT user_library_items_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


