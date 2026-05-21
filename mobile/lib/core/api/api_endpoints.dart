class ApiEndpoints {
  ApiEndpoints._();
  static const redeemInvite = '/redeem-invite';
  static const uploadsPresign = '/uploads/presign';
  static const generate = '/generate';
  static const jobs = '/jobs';
  static String job(String id) => '/jobs/$id';
}
