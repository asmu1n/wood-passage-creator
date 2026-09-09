declare namespace API {
  type BaseResponse<T = any> = {
    code?: number
    data?: T
    message?: string
  }

  type BaseResponseUser = BaseResponse<User>
  type BaseResponseString = BaseResponse<string>
  type BaseResponse = BaseResponse<null | Record<string, never> | any>
  type BaseResponseArticle = BaseResponse<Article>
  type BaseResponsePageArticle = BaseResponse<PageArticle>
  type BaseResponsePageUser = BaseResponse<PageUser>
  type BaseResponsePagePaymentRecord = BaseResponse<PagePaymentRecord>
  type BaseResponseListOutlineSection = BaseResponse<OutlineSection[]>
  type BaseResponseAgentExecutionStats = BaseResponse<AgentExecutionStats>
  type BaseResponseStatisticsVO = BaseResponse<StatisticsVO>
  type BaseResponseMockSessionResult = BaseResponse<MockSessionResult>
  type BaseResponseMockCompleteResult = BaseResponse<MockCompleteResult>

  /** @deprecated 兼容旧命名 */
  type BaseResponseArticleVO = BaseResponseArticle
  type BaseResponsePageArticleVO = BaseResponsePageArticle
  type BaseResponsePageUserVO = BaseResponsePageUser
  type BaseResponseLoginUserVO = BaseResponseUser
  type BaseResponseLong = BaseResponse<number>
  type BaseResponseBoolean = BaseResponse<boolean | null>
  type BaseResponseVoid = BaseResponse
  type BaseResponseListPaymentRecord = BaseResponsePagePaymentRecord
  type LoginUserVO = User
  type UserVO = User
  type ArticleVO = Article
  type PageArticleVO = PageArticle
  type PageUserVO = PageUser

  type PageQuery = {
    pageNum?: number
    pageSize?: number
    status?: string
    productType?: string
  }

  type PageArticle = {
    records?: Article[]
    total?: number
    pageSize?: number
    pageNum?: number
    /** @deprecated 旧字段，请用 total */
    totalRow?: number
    pageNumber?: number
  }

  type PageUser = {
    records?: User[]
    total?: number
    pageSize?: number
    pageNum?: number
    totalRow?: number
    pageNumber?: number
  }

  type PagePaymentRecord = {
    records?: PaymentRecord[]
    total?: number
    pageSize?: number
    pageNum?: number
  }

  type AgentExecutionStats = {
    taskId?: string
    totalDurationMs?: number
    agentCount?: number
    agentDurations?: Record<string, number>
    overallStatus?: string
    logs?: AgentLog[]
  }

  type AgentLog = {
    id?: number
    articleId?: number
    taskId?: string
    agentName?: string
    startTime?: string
    endTime?: string
    durationMs?: number
    status?: string
    errorMessage?: string
    prompt?: string
    inputData?: string
    outputData?: string
    createTime?: string
  }

  type ArticleAiModifyOutlineRequest = {
    taskId?: string
    modifySuggestion?: string
  }

  type ArticleConfirmOutlineRequest = {
    taskId?: string
    outline?: OutlineSection[]
  }

  type ArticleConfirmTitleRequest = {
    taskId?: string
    selectedMainTitle?: string
    selectedSubTitle?: string
    userDescription?: string
  }

  type ArticleCreateRequest = {
    topic?: string
    style?: string
    enabledImageMethods?: string[]
  }

  type ArticleQueryRequest = {
    pageNum?: number
    pageSize?: number
    status?: string
    userId?: number
    sortField?: string
    sortOrder?: string
  }

  type Article = {
    id?: number
    taskId?: string
    userId?: number
    topic?: string
    userDescription?: string
    mainTitle?: string
    subTitle?: string
    titleOptions?: TitleOption[]
    outline?: OutlineSection[]
    content?: string
    fullContent?: string
    images?: ImageItem[]
    status?: string
    phase?: string
    errorMessage?: string
    style?: string
    enabledImageMethods?: string[]
    createTime?: string
    completedTime?: string
    /** 旧字段，后端已无 */
    coverImage?: string
  }

  type DeleteRequest = {
    id?: number
  }

  type getArticleParams = { taskId: string }
  type getExecutionLogsParams = { taskId: string }
  type getProgressParams = { taskId: string }
  type getUserByIdParams = { id: number }

  type ImageItem = {
    position?: number
    url?: string
    method?: string
    keywords?: string
    sectionTitle?: string
    description?: string
    imageSource?: string
  }

  type OutlineSection = {
    section?: number
    title?: string
    points?: string[]
  }

  type OutlineItem = OutlineSection

  type PaymentRecord = {
    id?: number
    userId?: number
    stripeSessionId?: string
    stripePaymentIntentId?: string
    amount?: number
    currency?: string
    status?: string
    productType?: string
    description?: string
    createTime?: string
    updateTime?: string
  }

  type MockSessionResult = {
    sessionId?: string
    checkoutUrl?: string
    amount?: number
    currency?: string
    productType?: string
    status?: string
  }

  type MockCompleteRequest = {
    sessionId: string
  }

  type MockCompleteResult = {
    record?: PaymentRecord
    userId?: number
    isVip?: boolean
  }

  type StatisticsVO = {
    todayCount?: number
    weekCount?: number
    monthCount?: number
    totalCount?: number
    successRate?: number
    avgDurationMs?: number
    activeUserCount?: number
    totalUserCount?: number
    vipUserCount?: number
    quotaUsed?: number
  }

  type TitleOption = {
    mainTitle?: string
    subTitle?: string
  }

  type User = {
    id?: number
    userAccount?: string
    userName?: string
    userAvatar?: string
    userProfile?: string
    userRole?: string
    quota?: number
    vipTime?: string
    createTime?: string
    updateTime?: string
  }

  type UserLoginRequest = {
    userAccount?: string
    userPassword?: string
  }

  type UserQueryRequest = {
    pageNum?: number
    pageSize?: number
    userAccount?: string
    userName?: string
    userRole?: string
  }

  type UserRegisterRequest = {
    userAccount?: string
    userPassword?: string
    checkPassword?: string
  }

  type UserUpdateRequest = {
    userPassword?: string
    userName?: string
    userAvatar?: string
    userProfile?: string
  }
}
