/// Mirrors core-api's domain.User JSON shape exactly (see
/// backend/services/core-api/internal/domain/user.go).
class AppUser {
  final String id;
  final String tenantId;
  final String rauthySubjectId;
  final String displayName;
  final String email;
  final String role; // "admin" or "vendor"
  final DateTime createdAt;

  const AppUser({
    required this.id,
    required this.tenantId,
    required this.rauthySubjectId,
    required this.displayName,
    required this.email,
    required this.role,
    required this.createdAt,
  });

  bool get isAdmin => role == 'admin';

  factory AppUser.fromJson(Map<String, dynamic> json) {
    return AppUser(
      id: json['id'] as String,
      tenantId: json['tenant_id'] as String,
      rauthySubjectId: json['rauthy_subject_id'] as String,
      displayName: json['display_name'] as String,
      email: json['email'] as String,
      role: json['role'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }
}
