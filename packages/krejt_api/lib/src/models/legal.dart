/// Kushtet e përdorimit dhe Politika e privatësisë, ashtu si i kthen serveri: strukturë, jo HTML.
/// Aplikacioni i vizaton me stilin e vet, që një dokument ligjor të mos duket si faqe interneti
/// e futur me zor brenda ekranit.
class LegalSection {
  const LegalSection({required this.heading, required this.paragraphs});

  factory LegalSection.fromJson(Map<String, dynamic> j) => LegalSection(
    heading: j['heading'] as String? ?? '',
    paragraphs: ((j['paragraphs'] as List?) ?? const []).map((p) => p as String).toList(),
  );

  final String heading;
  final List<String> paragraphs;
}

class LegalDocument {
  const LegalDocument({
    required this.doc,
    required this.lang,
    required this.title,
    required this.updated,
    required this.sections,
  });

  factory LegalDocument.fromJson(Map<String, dynamic> j) => LegalDocument(
    doc: j['doc'] as String? ?? '',
    lang: j['lang'] as String? ?? 'sq',
    title: j['title'] as String? ?? '',
    updated: j['updated'] as String? ?? '',
    sections: ((j['sections'] as List?) ?? const [])
        .map((s) => LegalSection.fromJson(s as Map<String, dynamic>))
        .toList(),
  );

  final String doc;
  final String lang;
  final String title;

  /// Data e redaktimit (YYYY-MM-DD); përdoruesi ka të drejtë ta dijë cilën redaktim po lexon.
  final String updated;
  final List<LegalSection> sections;
}
