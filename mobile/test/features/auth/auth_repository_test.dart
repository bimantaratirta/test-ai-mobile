import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:ai_image/features/auth/auth_repository.dart';

class _MockDio extends Mock implements Dio {}

void main() {
  setUpAll(() {
    registerFallbackValue(Uri());
  });

  test('redeemInvite POSTs the code and returns true on 200', () async {
    final dio = _MockDio();
    final resp = Response<dynamic>(
      requestOptions: RequestOptions(path: '/redeem-invite'),
      data: {'redeemed': true},
      statusCode: 200,
    );
    when(() => dio.post<dynamic>(
          '/redeem-invite',
          data: any<dynamic>(named: 'data'),
        )).thenAnswer((_) async => resp);

    final repo = AuthRepository(dio: dio);
    final ok = await repo.redeemInvite('BETA001');
    expect(ok, isTrue);
    verify(() => dio.post<dynamic>(
          '/redeem-invite',
          data: {'code': 'BETA001'},
        )).called(1);
  });

  test('redeemInvite throws when status not 200', () async {
    final dio = _MockDio();
    when(() => dio.post<dynamic>(
          any(),
          data: any<dynamic>(named: 'data'),
        )).thenThrow(DioException(
      requestOptions: RequestOptions(path: '/redeem-invite'),
      response: Response<dynamic>(
        requestOptions: RequestOptions(path: '/redeem-invite'),
        data: {'error': 'invite code not found'},
        statusCode: 400,
      ),
    ));

    final repo = AuthRepository(dio: dio);
    await expectLater(
      () => repo.redeemInvite('NOPE'),
      throwsA(isA<Exception>()),
    );
  });
}
